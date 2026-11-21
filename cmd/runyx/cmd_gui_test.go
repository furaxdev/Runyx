package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
// generic "timed out" -- it must report the real cause.
//
// Asserts on the error's *content*, not wall-clock elapsed time: under
// `go test -race ./...`'s full-repo cross-package parallelism, a CI
// runner can be CPU-starved badly enough that even this test's own
// background goroutines don't get scheduled for 20-30+ real seconds
// -- not because anything is broken, but because dozens of other
// packages' race-instrumented tests are all competing for 2 vCPUs at
// once. That stretches every wall-clock duration in this test
// uniformly, including the "it already failed, just hasn't been
// scheduled to report it yet" case, so a duration threshold can't
// actually distinguish fixed-and-slow from broken-and-slow here. The
// error text can: the fixed code path's error always names the real
// daemon exit, the broken one always says "timed out", regardless of
// how long either took to surface under contention.
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

	stop, err := ensureDaemonRunning(cfg, "http://"+addr)
	if err == nil {
		if stop != nil {
			stop()
		}
		t.Fatal("expected an error when the listen address is already bound")
	}
	if strings.Contains(err.Error(), "timed out waiting for daemon to become ready") {
		t.Errorf("ensureDaemonRunning reported a generic timeout instead of the daemon's real startup error: %v", err)
	}
	if !strings.Contains(err.Error(), "exited before becoming ready") {
		t.Errorf("expected the error to report the daemon's own exit, got: %v", err)
	}
}
