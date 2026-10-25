package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/modelstore"
)

// TestRegistryPullSpecMatchesInstalledName guards against a real bug
// found via the demo screenshots: the CLI's `runyx pull owner/repo`
// (no explicit :file) registers the model under the bare repo name, but
// the registry previously always reported a synthesized "repo:file"
// pull_spec for installed entries. The web UI sends that pull_spec
// straight to /v1/chat/completions as the model name, so the mismatch
// 404'd every chat request for anything pulled without an explicit
// file — exactly what the real demo.yml run surfaced.
func TestRegistryPullSpecMatchesInstalledName(t *testing.T) {
	store, err := modelstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("modelstore.New: %v", err)
	}
	ggufPath := writeTempGGUF(t)
	const bareRepoName = "Qwen/Qwen2.5-0.5B-Instruct-GGUF"
	if _, err := store.ImportLocal(bareRepoName, ggufPath); err != nil {
		t.Fatalf("ImportLocal: %v", err)
	}

	mux := http.NewServeMux()
	s := &Server{Store: store, HW: hwdetect.Info{TotalRAMMB: 16384}}
	reg := &RegistryExtras{Server: s}
	reg.RegisterRegistry(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/runyx/registry", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/runyx/registry: status %d, body %s", rec.Code, rec.Body.String())
	}

	var entries []RegistryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var found bool
	for _, e := range entries {
		if e.Repo != bareRepoName {
			continue
		}
		found = true
		if !e.Installed {
			t.Fatalf("expected %s to be reported installed", bareRepoName)
		}
		if e.PullSpec != bareRepoName {
			t.Errorf("pull_spec = %q, want %q (the actual stored model name, not a synthesized repo:file)", e.PullSpec, bareRepoName)
		}
	}
	if !found {
		t.Fatalf("registry entry for %s not found", bareRepoName)
	}
}

func writeTempGGUF(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(path, []byte("fake gguf bytes"), 0o644); err != nil {
		t.Fatalf("write temp gguf: %v", err)
	}
	return path
}
