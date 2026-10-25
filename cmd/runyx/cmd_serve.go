package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/config"
	"github.com/furaxdev/runyx/internal/daemon"
	"github.com/furaxdev/runyx/internal/hwdetect"
)

func cmdServe(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}

	binPath := resolveBackendBinary(cfg)

	d, err := daemon.New(cfg, binPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("runyx daemon listening on http://%s (Ollama API at /api, OpenAI API at /v1)\n", cfg.ListenAddr)
	return d.Serve(ctx)
}

// resolveBackendBinary returns a cached llama-server path if one exists,
// without blocking `serve` startup on a network fetch. If nothing is
// cached, it returns "" — the daemon will serve list/show immediately,
// and the first attempt to run a model will surface a clear error telling
// the user to run `runyx doctor` (which triggers the fetch).
func resolveBackendBinary(cfg config.Config) string {
	if path := findCachedBackend(cfg.BackendsDir); path != "" {
		return path
	}
	info := hwdetect.Detect()
	fetcher := backend.NewFetcher(cfg.BackendsDir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	path, err := fetcher.Ensure(ctx, info, nil)
	if err != nil {
		return "" // no cached build and the network fetch didn't land in time
	}
	return path
}

// findCachedBackend looks for an already-extracted llama-server binary
// under any previously fetched release directory, so `serve` never blocks
// on the network when a backend is already installed.
func findCachedBackend(backendsDir string) string {
	name := "llama-server"
	if runtime.GOOS == "windows" {
		name = "llama-server.exe"
	}
	entries, err := os.ReadDir(backendsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(backendsDir, e.Name(), name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
