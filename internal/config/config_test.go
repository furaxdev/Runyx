package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:11535" {
		t.Errorf("ListenAddr = %q, want loopback default", cfg.ListenAddr)
	}
	if cfg.Telemetry {
		t.Error("Telemetry must default to false")
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %s: %v", path, err)
	}

	cfg.Telemetry = true // should be forced back to false on next Load
	cfg.VRAMBudgetMB = 4096
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Telemetry {
		t.Error("Telemetry must never load as true")
	}
	if reloaded.VRAMBudgetMB != 4096 {
		t.Errorf("VRAMBudgetMB = %d, want 4096", reloaded.VRAMBudgetMB)
	}
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		ModelsDir:   filepath.Join(dir, "models"),
		BackendsDir: filepath.Join(dir, "backends"),
		DataDir:     filepath.Join(dir, "data"),
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	for _, d := range []string{cfg.ModelsDir, cfg.BackendsDir, cfg.DataDir} {
		if info, err := os.Stat(d); err != nil || !info.IsDir() {
			t.Errorf("expected dir %s to exist", d)
		}
	}
}
