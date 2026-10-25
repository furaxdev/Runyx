package main

import (
	"context"
	"fmt"
	"time"

	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/bench"
	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/modelstore"
)

// cmdBench loads a model (if not already loaded) and measures its
// tokens/sec and time-to-first-token on a fixed prompt.
func cmdBench(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx bench <model> [prompt]")
	}
	name := args[0]
	prompt := "Explain in two sentences what a large language model is."
	if len(args) > 1 {
		prompt = args[1]
	}

	cfg, store, err := loadStore()
	if err != nil {
		return err
	}
	mdl, ok, err := store.Get(name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("model %q not found; try `runyx list`", name)
	}

	binPath, err := resolveBackendBinaryBlocking(cfg, func(p downloader.Progress) {
		printProgress(p)
	})
	if err != nil {
		fmt.Println()
		return fmt.Errorf("fetch llama-server backend: %w", err)
	}
	fmt.Println()

	mgr := backend.NewManager(binPath, 0, cfg.VRAMBudgetMB)
	defer mgr.StopAll()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	inst, err := mgr.Start(ctx, mdl, tuningOrDefault(mdl))
	if err != nil {
		return fmt.Errorf("start model: %w", err)
	}

	fmt.Printf("Benchmarking %s...\n", name)
	result, err := bench.Run(ctx, nil, inst.BaseURL, name, prompt)
	if err != nil {
		return err
	}
	fmt.Printf("Time to first token: %s\n", result.TimeToFirstTok)
	fmt.Printf("Total duration:      %s\n", result.TotalDuration)
	fmt.Printf("Tokens generated:    %d\n", result.TokenCount)
	fmt.Printf("Tokens/sec:          %.1f\n", result.TokensPerSec)
	return nil
}

func tuningOrDefault(mdl modelstore.Model) modelstore.TuningParams {
	if mdl.Tuning != (modelstore.TuningParams{}) {
		return mdl.Tuning
	}
	return modelstore.TuningParams{CtxSize: 4096, BatchSize: 512, Threads: 4}
}
