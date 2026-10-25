package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/furaxdev/runyx/internal/mcp"
)

// TestMCPServeRealSubprocess builds this very command and runs it as a
// real OS subprocess speaking MCP over its actual stdin/stdout pipes —
// not an in-memory io.Pipe — to prove `runyx mcp-serve` genuinely works
// as an external tool server, which is the point of item 10's "mode
// serveur MCP".
func TestMCPServeRealSubprocess(t *testing.T) {
	bin := buildTestBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := mcp.StartProcess(ctx, bin, "mcp-serve")
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}
	defer client.Close()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	found := false
	for _, tool := range tools {
		if tool.Name == "web_search" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a web_search tool, got %v", tools)
	}

	// Exercise the real tool end to end, against the real network, with
	// retries for this sandbox's known-flaky egress to DuckDuckGo (see
	// internal/websearch's own tests for the same pattern).
	var result string
	for attempt := 0; attempt < 4; attempt++ {
		result, err = client.CallTool(ctx, "web_search", map[string]any{"query": "golang programming", "max_results": 2.0})
		if err == nil && strings.TrimSpace(result) != "" {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		t.Fatalf("CallTool(web_search): %v", err)
	}
	if !strings.Contains(result, "untrusted external content") {
		t.Errorf("expected the web search tool result to self-label as untrusted, got: %s", result)
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	bin := t.TempDir() + "/runyx-mcp-test-bin"
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build test binary: %v\n%s", err, out)
	}
	return bin
}
