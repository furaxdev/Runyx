package api

import (
	"context"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/furaxdev/runyx/internal/plugin"
)

func buildTestRunyxBinary(t *testing.T) string {
	t.Helper()
	bin := t.TempDir() + "/runyx-m6-test-bin"
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/runyx")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build runyx binary: %v\n%s", err, out)
	}
	return bin
}

func TestPluginsListAndCallOverHTTP(t *testing.T) {
	bin := buildTestRunyxBinary(t)
	pm := plugin.NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := pm.Start(ctx, plugin.Spec{Name: "web", Command: bin, Args: []string{"mcp-serve"}}); err != nil {
		t.Fatalf("Start plugin: %v", err)
	}
	defer pm.StopAll()

	mux := http.NewServeMux()
	m := &M6Extras{Plugins: pm}
	m.RegisterM6(mux)

	rec := doJSON(t, mux, "GET", "/api/runyx/plugins", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "web_search") {
		t.Errorf("body = %s, expected it to list the web_search tool", rec.Body.String())
	}
}

func TestPluginsEmptyWithoutManager(t *testing.T) {
	mux := http.NewServeMux()
	m := &M6Extras{}
	m.RegisterM6(mux)
	rec := doJSON(t, mux, "GET", "/api/runyx/plugins", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "null\n" {
		t.Errorf("status=%d body=%q, want 200 and an empty/null list", rec.Code, rec.Body.String())
	}
}

func TestArenaRequiresAtLeastTwoModels(t *testing.T) {
	mux := http.NewServeMux()
	s := &Server{}
	m := &M6Extras{Server: s}
	m.RegisterM6(mux)
	rec := doJSON(t, mux, "POST", "/api/runyx/arena", map[string]any{"Models": []string{"only-one"}})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
