package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/furaxdev/runyx/internal/colibri"
	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
)

// cmdGPUSaver handles `runyx gpu-saver status|import`. GPU Saver
// (colibri) is never fetched or run automatically -- see
// internal/colibri's package doc and docs/DECISIONS.md -- so these are
// the only two places that actually download it or convert a model for
// it.
func cmdGPUSaver(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx gpu-saver <status|import> ...")
	}
	switch args[0] {
	case "status":
		return cmdGPUSaverStatus(args[1:])
	case "import":
		return cmdGPUSaverImport(args[1:])
	default:
		return fmt.Errorf("unknown gpu-saver subcommand %q (want status or import)", args[0])
	}
}

func cmdGPUSaverStatus(args []string) error {
	cfg, store, err := loadStore()
	if err != nil {
		return err
	}
	hw := hwdetect.Detect()

	fmt.Printf("GPU Saver enabled in config.toml: %v\n", cfg.GPUSaverEnabled)
	fetcher := colibri.NewFetcher(filepath.Join(cfg.BackendsDir, "colibri"))
	if launcherPath, ok := fetcher.CachedLauncher(hw); ok {
		fmt.Printf("colibri: fetched, at %s\n", launcherPath)
	} else {
		fmt.Println("colibri: not fetched yet (`runyx gpu-saver import <model>` fetches it)")
	}
	if _, err := colibri.FindPython(); err != nil {
		fmt.Printf("warning: %v\n", err)
	}

	if len(args) == 0 {
		return nil
	}
	mdl, ok, err := store.Get(args[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("model not found: %s", args[0])
	}
	rec := colibri.Recommended(hw, mdl.SizeBytes, mdl.Name+" "+mdl.Repo)
	fmt.Printf("\n%s: recommended = %v\n  %s\n", mdl.Name, rec.Recommended, rec.Reason)
	return nil
}

func cmdGPUSaverImport(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx gpu-saver import <model>")
	}
	cfg, store, err := loadStore()
	if err != nil {
		return err
	}
	mdl, ok, err := store.Get(args[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("model not found: %s", args[0])
	}

	hw := hwdetect.Detect()
	fetcher := colibri.NewFetcher(filepath.Join(cfg.BackendsDir, "colibri"))

	fmt.Println("Fetching colibri (first time only)...")
	ctx := context.Background()
	launcherPath, err := fetcher.Ensure(ctx, hw, func(p downloader.Progress) {
		if p.Total > 0 {
			fmt.Printf("\r  %d / %d MB", p.Downloaded/(1024*1024), p.Total/(1024*1024))
		}
	})
	if err != nil {
		return fmt.Errorf("fetch colibri: %w", err)
	}
	fmt.Println()

	pythonPath, err := colibri.FindPython()
	if err != nil {
		return err
	}

	outDir := colibri.ImportedModelDir(filepath.Dir(launcherPath), filepath.Base(mdl.File))
	if _, statErr := os.Stat(outDir); statErr == nil {
		fmt.Printf("%s is already imported at %s\n", mdl.Name, outDir)
		return nil
	}

	// A real conversion of a large model can take a long time -- this
	// deliberately streams coli's own output straight to the terminal
	// (the same "visible, can take a while" contract `runyx pull`
	// already has) rather than hiding it behind a spinner.
	fmt.Printf("Converting %s for colibri — this can take a while for a large model...\n", mdl.Name)
	if err := colibri.RunImport(ctx, pythonPath, launcherPath, mdl.File, outDir, os.Stdout, os.Stderr); err != nil {
		return err
	}
	fmt.Printf("Done. %s will now be served via colibri (GPU Saver) once GPU Saver is enabled in config.toml.\n", mdl.Name)
	return nil
}
