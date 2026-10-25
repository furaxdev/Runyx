package modelstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OllamaHome returns Ollama's default model storage root for the current
// OS/user, mirroring Ollama's own documented default (not its source):
// $OLLAMA_MODELS if set, else ~/.ollama/models.
func OllamaHome() string {
	if dir := os.Getenv("OLLAMA_MODELS"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ollama", "models")
}

// ollamaManifestLayer mirrors the subset of Ollama's manifest JSON shape
// (per its publicly documented registry format) that we need: a list of
// layers, each with a media type and a blob digest.
type ollamaManifestLayer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ollamaManifest struct {
	Layers []ollamaManifestLayer `json:"layers"`
}

// ScanOllamaModels walks an Ollama models directory
// (<root>/manifests/<registry>/<namespace>/<name>/<tag>) and returns, for
// each manifest found, the resolved path of its GGUF blob
// (<root>/blobs/sha256-<digest>) along with a Runyx-style name
// "<namespace>/<name>:<tag>" (namespace dropped when it's "library").
func ScanOllamaModels(root string) ([]OllamaFound, error) {
	manifestsDir := filepath.Join(root, "manifests")
	var found []OllamaFound
	err := filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries, best-effort scan
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(manifestsDir, path)
		if relErr != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		// expected: registry/namespace/name/tag
		if len(parts) < 4 {
			return nil
		}
		tag := parts[len(parts)-1]
		name := parts[len(parts)-2]
		namespace := parts[len(parts)-3]

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var mf ollamaManifest
		if jsonErr := json.Unmarshal(data, &mf); jsonErr != nil {
			return nil
		}
		for _, layer := range mf.Layers {
			if layer.MediaType != "application/vnd.ollama.image.model" {
				continue
			}
			digest := strings.TrimPrefix(layer.Digest, "sha256:")
			blobPath := filepath.Join(root, "blobs", "sha256-"+digest)
			if _, statErr := os.Stat(blobPath); statErr != nil {
				continue
			}
			displayName := name
			if namespace != "library" {
				displayName = namespace + "/" + name
			}
			found = append(found, OllamaFound{
				Name:      fmt.Sprintf("%s:%s", displayName, tag),
				BlobPath:  blobPath,
				SizeBytes: layer.Size,
				Digest:    digest,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// OllamaFound is one model discovered in an Ollama installation.
type OllamaFound struct {
	Name      string
	BlobPath  string
	SizeBytes int64
	Digest    string
}

// ImportFromOllama registers an Ollama-discovered model in the Runyx
// store, referencing the existing blob file in place (no copy, no
// re-download).
func (s *Store) ImportFromOllama(f OllamaFound) (Model, error) {
	mdl := Model{
		Name:       "ollama/" + f.Name,
		File:       f.BlobPath,
		SizeBytes:  f.SizeBytes,
		SHA256:     f.Digest,
		Source:     "ollama",
		ImportedAt: time.Now(),
	}
	if err := s.Put(mdl); err != nil {
		return Model{}, err
	}
	return mdl, nil
}
