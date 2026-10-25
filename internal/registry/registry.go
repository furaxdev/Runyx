// Package registry loads Runyx's curated model catalog: a versioned,
// hand-picked list of real Hugging Face GGUF repos (not a live search),
// so the UI's Models page can show a name, description and capability
// badges instead of a bare "type a repo name" field.
//
// registry.json is authored at models/registry.json (the editable
// source of truth) and embedded here as a committed copy — the same
// pattern internal/webui uses for the built web UI — kept in sync by
// scripts/sync-registry.sh. Run that script and commit the result after
// editing models/registry.json.
//
// Every entry here is a real, currently-resolvable Hugging Face repo +
// file: see cmd/runyx/cmd_verify_registry.go (run via `go run
// ./cmd/verify-registry` and in CI) which HEADs each entry's download
// URL and fails if any file is missing or its size drifted.
package registry

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed registry.json
var embedded embed.FS

// Model is one catalog entry.
type Model struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Publisher     string   `json:"publisher"`
	Family        string   `json:"family"`
	ParamsB       float64  `json:"params_b"`
	Repo          string   `json:"repo"`
	File          string   `json:"file"`
	SizeBytes     int64    `json:"size_bytes"`
	License       string   `json:"license"`
	MMProjFile    string   `json:"mmproj_file,omitempty"` // vision models' projector file, if any
	HFURL         string   `json:"hf_url"`
	Description   string   `json:"description"`
	Capabilities  []string `json:"capabilities"`
	ContextLength int      `json:"context_length"`
}

// PullSpec returns the exact "owner/repo:file.gguf" argument `runyx
// pull` (and internal/downloader.FileURL) expects for this entry.
func (m Model) PullSpec() string {
	return m.Repo + ":" + m.File
}

// Registry is the parsed catalog.
type Registry struct {
	SchemaVersion int     `json:"schema_version"`
	UpdatedAt     string  `json:"updated_at"`
	Models        []Model `json:"models"`
}

// Load parses the embedded registry.json.
func Load() (Registry, error) {
	data, err := embedded.ReadFile("registry.json")
	if err != nil {
		return Registry{}, fmt.Errorf("read embedded registry.json: %w", err)
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return Registry{}, fmt.Errorf("parse registry.json: %w", err)
	}
	return reg, nil
}
