package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
// opening again: a lock file whose daemon doesn't actually answer must
// be treated as stale and cleaned up, not as "someone else is running".
func TestAcquireSingleInstanceRecoversFromStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "gui.lock")
	if err := os.WriteFile(lockPath, []byte("99999\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	won, err := acquireSingleInstance(lockPath, "127.0.0.1:1") // nothing listens on port 1
	if err != nil {
		t.Fatalf("acquireSingleInstance: %v", err)
	}
	if !won {
		t.Error("expected to win and recover from a stale lock file whose daemon is unreachable")
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
