// Package config handles Runyx's on-disk configuration: where models and
// state live, which address the daemon listens on, and per-model tuning
// overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// Config is the root on-disk configuration, stored as TOML.
type Config struct {
	// ListenAddr is the host:port the daemon's HTTP API binds to.
	// Loopback-only by default; binding elsewhere is an explicit opt-in.
	ListenAddr string `toml:"listen_addr"`
	// ModelsDir is where downloaded/imported GGUF files are stored.
	ModelsDir string `toml:"models_dir"`
	// BackendsDir is where downloaded llama.cpp builds are stored.
	BackendsDir string `toml:"backends_dir"`
	// DataDir holds the SQLite state database.
	DataDir string `toml:"data_dir"`
	// Telemetry is always false and cannot be turned on; kept explicit in
	// the file so the choice is visible to the user.
	Telemetry bool `toml:"telemetry"`
	// IdleUnloadSeconds is how long an idle model stays loaded before
	// eviction. 0 disables idle unloading.
	IdleUnloadSeconds int `toml:"idle_unload_seconds"`
	// VRAMBudgetMB caps total VRAM Runyx will try to use across loaded
	// models. 0 means "auto-detect".
	VRAMBudgetMB int `toml:"vram_budget_mb"`

	// AgentWorkDir, if set, enables the safe shell agent (item 21)
	// scoped to this directory. Empty (the default) disables it
	// entirely — Runyx never runs shell commands out of the box.
	AgentWorkDir string `toml:"agent_work_dir"`
	// InterpreterWorkDir, if set, enables the sandboxed code
	// interpreter (item 22) scoped to this directory. Empty (the
	// default) disables it entirely, for the same reason.
	InterpreterWorkDir string `toml:"interpreter_work_dir"`
}

// Default returns the configuration used when no config file exists yet.
func Default() Config {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = "."
	}
	data, err := userDataDir()
	if err != nil || data == "" {
		data = filepath.Join(base, "runyx")
	}
	return Config{
		ListenAddr:        "127.0.0.1:11535",
		ModelsDir:         filepath.Join(data, "models"),
		BackendsDir:       filepath.Join(data, "backends"),
		DataDir:           data,
		Telemetry:         false,
		IdleUnloadSeconds: 600,
		VRAMBudgetMB:      0,
	}
}

// Dir returns the directory the config file lives in.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(base, "runyx"), nil
}

// Path returns the full path to config.toml.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads the config file, creating it with defaults if absent.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := Save(cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.Telemetry = false // non-negotiable, regardless of what's on disk
	return cfg, nil
}

// Save writes the config file, creating parent directories as needed.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// EnsureDirs creates the models/backends/data directories if missing.
func (c Config) EnsureDirs() error {
	for _, d := range []string{c.ModelsDir, c.BackendsDir, c.DataDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return nil
}

func userDataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", fmt.Errorf("LOCALAPPDATA not set")
		}
		return filepath.Join(base, "Runyx"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "runyx"), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "runyx"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "runyx"), nil
	}
}
