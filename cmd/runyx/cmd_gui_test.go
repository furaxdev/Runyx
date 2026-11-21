package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/furaxdev/runyx/internal/config"
)

// TestAcquireSingleInstanceFirstCallerWins guards the common case: no
// lock file exists yet, so the first process to call this should win
// outright without needing to probe any daemon.
func TestAcquireSingleInstanceFirstCallerWins(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "gui.lock")
	won, err := acquireSingleInstance(lockPath, "127.0.0.1:1") // nothing listens on port 1
	if err != nil {
		t.Fatalf("acquireSingleInstance: %v", err)
	}
	if !won {
		t.Error("expected the first caller to win with no pre-existing lock file")
	}
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Errorf("expected the lock file to exist after winning, stat err: %v", statErr)
	}
}

// TestAcquireSingleInstanceLosesToReachableDaemon guards the actual
// "only one Runyx window" requirement: a stale-looking lock file
// (already present) backed by a daemon that really does answer means
// a genuine instance is running, so this caller must lose.
func TestAcquireSingleInstanceLosesToReachableDaemon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	addr := srv.Listener.Addr().String()

	lockPath := filepath.Join(t.TempDir(), "gui.lock")
	if err := os.WriteFile(lockPath, []byte("12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	won, err := acquireSingleInstance(lockPath, addr)
	if err != nil {
		t.Fatalf("acquireSingleInstance: %v", err)
	}
	if won {
		t.Error("expected to lose against a lock file backed by a reachable daemon")
	}
}

// TestAcquireSingleInstanceRecoversFromStaleLock guards against a
// crashed previous instance permanently locking Runyx out of ever
// opening again: a lock file old enough that any legitimate startup
// would have finished, whose daemon doesn't actually answer, must be
// treated as stale and cleaned up, not as "someone else is running".
func TestAcquireSingleInstanceRecoversFromStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "gui.lock")
	if err := os.WriteFile(lockPath, []byte("99999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	backdateLock(t, lockPath, staleLockGracePeriod+time.Second)

	won, err := acquireSingleInstance(lockPath, "127.0.0.1:1") // nothing listens on port 1
	if err != nil {
		t.Fatalf("acquireSingleInstance: %v", err)
	}
	if !won {
		t.Error("expected to win and recover from a stale lock file whose daemon is unreachable")
	}
}

// TestAcquireSingleInstanceDoesNotStealLockDuringStartup guards the
// real race ensureDaemonRunning's up-to-10s startup window creates: a
// freshly created lock file backed by a daemon that isn't reachable
// *yet* must not be mistaken for a crash. Stealing it here would let a
// second process bind the same listen address the first is still
// starting up on, instead of reporting "Runyx is already running."
func TestAcquireSingleInstanceDoesNotStealLockDuringStartup(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "gui.lock")
	if err := os.WriteFile(lockPath, []byte("99999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Lock file's mtime is "now" (just created), well inside the grace
	// period -- simulates another instance that is still mid-startup.

	won, err := acquireSingleInstance(lockPath, "127.0.0.1:1") // its daemon isn't up yet
	if err != nil {
		t.Fatalf("acquireSingleInstance: %v", err)
	}
	if won {
		t.Error("expected to lose against a fresh lock file whose owner may still be starting its daemon")
	}
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Errorf("expected the original lock file to survive untouched, stat err: %v", statErr)
	}
}

// backdateLock sets lockPath's modification time to age in the past,
// simulating a lock file created that long ago.
func backdateLock(t *testing.T, lockPath string, age time.Duration) {
	t.Helper()
	past := time.Now().Add(-age)
	if err := os.Chtimes(lockPath, past, past); err != nil {
		t.Fatalf("backdate lock file: %v", err)
	}
}

func TestDaemonReachableTrueForRealServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if !daemonReachable(srv.Listener.Addr().String()) {
		t.Error("expected daemonReachable to be true for a real, answering server")
	}
}

func TestDaemonReachableFalseWhenNothingListens(t *testing.T) {
	if daemonReachable("127.0.0.1:1") {
		t.Error("expected daemonReachable to be false when nothing listens on the port")
	}
}

// TestEnsureDaemonRunningFailsFastWhenPortInUse guards against
// ensureDaemonRunning silently discarding a real startup error (e.g.
// the listen address already being bound by something else) and
// instead burning the full 10s readiness timeout before reporting a
// generic "timed out" -- it must return promptly with the real cause.
func TestEnsureDaemonRunningFailsFastWhenPortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	dir := t.TempDir()
	cfg := config.Config{
		ListenAddr:  addr,
		ModelsDir:   filepath.Join(dir, "models"),
		BackendsDir: filepath.Join(dir, "backends"),
		DataDir:     filepath.Join(dir, "data"),
	}

	start := time.Now()
	stop, err := ensureDaemonRunning(cfg, "http://"+addr)
	elapsed := time.Since(start)
	if err == nil {
		if stop != nil {
			stop()
		}
		t.Fatal("expected an error when the listen address is already bound")
	}
	// daemon.New() (called synchronously, before the race-the-readiness
	// logic this test is actually guarding even starts) runs
	// hwdetect.Detect(), which is now bounded by gpuProbeTimeout per
	// vendor-tool probe (see internal/hwdetect/gpu.go) -- worst case
	// two sequential probes on Linux/Windows. 15s gives that real bound
	// comfortable margin for CI scheduling jitter while still catching
	// a real regression back to the old "waits out the full 10s+
	// readiness timeout" behavior this test exists to guard against.
	if elapsed > 15*time.Second {
		t.Errorf("ensureDaemonRunning took %s to fail -- looks like it waited out the full readiness timeout instead of catching the daemon's own startup error", elapsed)
	}
}
