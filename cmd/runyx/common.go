package main

import (
	"context"
	"fmt"
	"time"

	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/config"
	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/modelstore"
)

// loadStore loads the config and opens the model store, creating
// directories as needed. Shared by every subcommand that touches
// installed models.
func loadStore() (config.Config, *modelstore.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("load config: %w", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		return config.Config{}, nil, err
	}
	store, err := modelstore.New(cfg.ModelsDir)
	if err != nil {
		return config.Config{}, nil, err
	}
	return cfg, store, nil
}

// resolveBackendBinaryBlocking is resolveBackendBinary's counterpart for
// commands that the user explicitly asked to run a model (`run`,
// `bench`): unlike `serve` on startup, these should actually wait for a
// real network fetch instead of giving up after 5 seconds, the same way
// `pull` already blocks for the model weights themselves.
func resolveBackendBinaryBlocking(cfg config.Config, onProgress downloader.ProgressFunc) (string, error) {
	if path := findCachedBackend(cfg.BackendsDir); path != "" {
		return path, nil
	}
	info := hwdetect.Detect()
	fetcher := backend.NewFetcher(cfg.BackendsDir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return fetcher.Ensure(ctx, info, onProgress)
}
