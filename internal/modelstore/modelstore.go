// Package modelstore manages Runyx's local registry of GGUF models:
// what's installed, where the file lives, and how it was tuned. The
// registry itself is a small JSON manifest next to the model files — no
// need for SQLite just to list files on disk.
package modelstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Model is one installed GGUF model.
type Model struct {
	Name       string    `json:"name"`  // user-facing name, e.g. "llama3.1:8b"
	Repo       string    `json:"repo"`  // Hugging Face repo id, empty if imported
	File       string    `json:"file"`  // absolute path to the .gguf file
	Quant      string    `json:"quant"` // e.g. "Q4_K_M"
	SizeBytes  int64     `json:"size_bytes"`
	SHA256     string    `json:"sha256"`
	Source     string    `json:"source"` // "huggingface" | "local" | "ollama"
	ImportedAt time.Time `json:"imported_at"`

	// NumGPULayers etc. carry the last-tuned parameters for this model,
	// so `runyx run` doesn't have to retune every time.
	Tuning TuningParams `json:"tuning,omitempty"`
}

// TuningParams are the llama-server flags cached per model.
type TuningParams struct {
	NGPULayers int  `json:"ngpu_layers"`
	Threads    int  `json:"threads"`
	CtxSize    int  `json:"ctx_size"`
	BatchSize  int  `json:"batch_size"`
	FlashAttn  bool `json:"flash_attn"`
}

// manifest is the on-disk JSON format.
type manifest struct {
	Models []Model `json:"models"`
}

// Store manages the model registry rooted at a models directory.
type Store struct {
	dir string
}

// New returns a Store rooted at dir, creating the directory if needed.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create models dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) manifestPath() string {
	return filepath.Join(s.dir, "models.json")
}

// DestPath returns the on-disk path a pulled repo:file would be saved
// to, the single shared naming scheme used by both `runyx pull` and the
// registry-driven server pull endpoint so they never disagree about
// where a given download lands.
func (s *Store) DestPath(repo, file string) string {
	return filepath.Join(s.dir, sanitizeFilename(repo)+"__"+file)
}

func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '/' {
			out = append(out, '_')
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}

func (s *Store) load() (manifest, error) {
	data, err := os.ReadFile(s.manifestPath())
	if os.IsNotExist(err) {
		return manifest{}, nil
	}
	if err != nil {
		return manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return m, nil
}

func (s *Store) save(m manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	tmp := s.manifestPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return os.Rename(tmp, s.manifestPath())
}

// List returns all registered models, sorted by name.
func (s *Store) List() ([]Model, error) {
	m, err := s.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(m.Models, func(i, j int) bool { return m.Models[i].Name < m.Models[j].Name })
	return m.Models, nil
}

// Get returns the model registered under name, or ok=false.
func (s *Store) Get(name string) (Model, bool, error) {
	models, err := s.List()
	if err != nil {
		return Model{}, false, err
	}
	for _, mdl := range models {
		if mdl.Name == name {
			return mdl, true, nil
		}
	}
	return Model{}, false, nil
}

// Put inserts or replaces a model entry by name.
func (s *Store) Put(mdl Model) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	replaced := false
	for i, existing := range m.Models {
		if existing.Name == mdl.Name {
			m.Models[i] = mdl
			replaced = true
			break
		}
	}
	if !replaced {
		m.Models = append(m.Models, mdl)
	}
	return s.save(m)
}

// Remove deletes a model's registry entry and, if removeFile is true, its
// backing GGUF file on disk.
func (s *Store) Remove(name string, removeFile bool) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	var kept []Model
	var removed *Model
	for _, mdl := range m.Models {
		if mdl.Name == name {
			removed = &mdl
			continue
		}
		kept = append(kept, mdl)
	}
	if removed == nil {
		return fmt.Errorf("model %q not found", name)
	}
	m.Models = kept
	if err := s.save(m); err != nil {
		return err
	}
	if removeFile {
		if err := os.Remove(removed.File); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove model file: %w", err)
		}
	}
	return nil
}

// SHA256File computes the SHA256 of a file on disk.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ImportLocal registers an existing local GGUF file under name, without
// copying it (Runyx references the file in place).
func (s *Store) ImportLocal(name, path string) (Model, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Model{}, fmt.Errorf("stat %s: %w", path, err)
	}
	if filepath.Ext(path) != ".gguf" {
		return Model{}, fmt.Errorf("%s is not a .gguf file", path)
	}
	sum, err := SHA256File(path)
	if err != nil {
		return Model{}, fmt.Errorf("checksum %s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Model{}, err
	}
	mdl := Model{
		Name:       name,
		File:       abs,
		SizeBytes:  info.Size(),
		SHA256:     sum,
		Source:     "local",
		ImportedAt: time.Now(),
	}
	if err := s.Put(mdl); err != nil {
		return Model{}, err
	}
	return mdl, nil
}
