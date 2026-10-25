package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/furaxdev/runyx/internal/config"
	"github.com/furaxdev/runyx/internal/daemon"
	"github.com/furaxdev/runyx/internal/guiwindow"
)

// cmdGUI is what a bare `runyx` (double-clicked or run with no
// arguments) does: make sure exactly one daemon is running on
// 127.0.0.1, then show Runyx's web UI in an app-style window pointed
// at it. `runyx serve` still exists for the old headless/server use
// case, and `runyx tui` for the terminal UI; this is the new default.
func cmdGUI(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}

	lockPath := filepath.Join(cfg.DataDir, "gui.lock")
	won, err := acquireSingleInstance(lockPath, cfg.ListenAddr)
	if err != nil {
		return err
	}
	if !won {
		fmt.Println("Runyx is already running.")
		return nil
	}
	defer os.Remove(lockPath)

	url := "http://" + cfg.ListenAddr

	stopDaemon, err := ensureDaemonRunning(cfg, url)
	if err != nil {
		return err
	}
	defer stopDaemon()

	return guiwindow.Open(url, "Runyx")
}

// acquireSingleInstance reports whether this process should proceed as
// the one-and-only Runyx GUI instance. It wins outright if it's the
// first to atomically create lockPath (O_EXCL makes "did I create this
// file or did it already exist" race-free across processes); if the
// file already exists, it checks whether a daemon is actually
// reachable at listenAddr before deciding what that means: a reachable
// daemon means a real instance (this one or a plain `runyx serve`) is
// genuinely running, so this process should not proceed. An
// unreachable daemon means the lock file is stale -- left behind by a
// crash, not a graceful shutdown that would have removed it -- and is
// safe to clean up and retry once.
func acquireSingleInstance(lockPath, listenAddr string) (won bool, err error) {
	if createLockFile(lockPath) {
		return true, nil
	}

	if daemonReachable(listenAddr) {
		return false, nil
	}

	// Stale lock from a process that didn't exit cleanly.
	_ = os.Remove(lockPath)
	if createLockFile(lockPath) {
		return true, nil
	}
	// Lost a race against another process retrying at the same moment;
	// treat that the same as "already running" rather than erroring.
	return false, nil
}

func createLockFile(path string) bool {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	fmt.Fprintf(f, "%d\n", os.Getpid())
	f.Close()
	return true
}

// daemonReachable reports whether a Runyx daemon is already listening
// and answering at listenAddr. Used both to decide against a second
// single-instance winner and to decide whether cmdGUI needs to start
// its own daemon at all.
func daemonReachable(listenAddr string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://" + listenAddr + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ensureDaemonRunning connects to an already-running daemon if one
// answers at cfg.ListenAddr, or starts one in-process otherwise. The
// returned stop func is a no-op in the "connected to an existing one"
// case -- a window that merely attached to someone else's daemon has
// no business shutting it down when that window closes -- and
// otherwise cancels the daemon's context and waits for it to shut down
// cleanly.
func ensureDaemonRunning(cfg config.Config, url string) (stop func(), err error) {
	if daemonReachable(cfg.ListenAddr) {
		return func() {}, nil
	}

	binPath := resolveBackendBinary(cfg)
	d, err := daemon.New(cfg, binPath)
	if err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Serve(ctx) }()

	if err := waitForDaemon(url, 10*time.Second); err != nil {
		cancel()
		<-done
		return nil, fmt.Errorf("daemon did not become ready: %w", err)
	}

	stop = func() {
		cancel()
		<-done
	}
	return stop, nil
}

// waitForDaemon polls url's health until it answers or timeout passes,
// so the window is only opened once there's actually something at the
// other end of it instead of racing the daemon's own startup.
func waitForDaemon(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/api/tags")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("timed out waiting for daemon to become ready")
}
