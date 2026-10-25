package plugin

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestPluginLifecycleWithRealSubprocess uses `runyx mcp-serve` (built
// fresh from cmd/runyx) as a stand-in plugin: a real external process
// speaking real MCP over its real stdin/stdout, launched and managed by
// this package exactly as a real plugin would be. This proves the
// plugin system works end to end, not just against mocks.
func TestPluginLifecycleWithRealSubprocess(t *testing.T) {
	bin := buildRunyxBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	m := NewManager()
	spec := Spec{Name: "test-plugin", Command: bin, Args: []string{"mcp-serve"}}

	running, err := m.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.StopAll()

	found := false
	for _, tool := range running.Tools {
		if tool.Name == "web_search" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the web_search tool, got %v", running.Tools)
	}

	// Starting the same plugin again must reuse the running instance,
	// not spawn a second subprocess.
	again, err := m.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start (again): %v", err)
	}
	if again != running {
		t.Error("expected Start to return the already-running instance")
	}

	if len(m.List()) != 1 {
		t.Errorf("List() = %v, want exactly 1 running plugin", m.List())
	}

	if err := m.Stop("test-plugin"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(m.List()) != 0 {
		t.Errorf("expected no running plugins after Stop, got %v", m.List())
	}
}

func TestCallUnknownPlugin(t *testing.T) {
	m := NewManager()
	_, err := m.Call(context.Background(), "nope", "tool", nil)
	if err == nil {
		t.Error("expected error calling a tool on a non-running plugin")
	}
}

func buildRunyxBinary(t *testing.T) string {
	t.Helper()
	bin := t.TempDir() + "/runyx-plugin-test-bin"
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/runyx")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build runyx binary: %v\n%s", err, out)
	}
	return bin
}
