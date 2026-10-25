package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/approval"
	"github.com/furaxdev/runyx/internal/interpreter"
)

func TestAgentExecuteDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	a := &AgentExtras{}
	a.RegisterAgent(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/agent/execute", map[string]string{"Command": "echo", "Reason": "test"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when no agent is configured", rec.Code)
	}
}

func TestInterpreterRunOverHTTP(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	mux := http.NewServeMux()
	a := &AgentExtras{Interpreter: &interpreter.Interpreter{WorkDir: t.TempDir()}}
	a.RegisterAgent(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/interpreter/run", map[string]string{"Code": "print('ok depuis http')"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ok depuis http") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestAgentExecuteOverHTTP(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	ag := agent.New(agent.Policy{WorkDir: dir}, func(context.Context, agent.Request) (bool, agent.ApprovalScope) {
		return true, agent.ApprovalOnce
	})
	mux := http.NewServeMux()
	a := &AgentExtras{Agent: ag}
	a.RegisterAgent(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/agent/execute", map[string]any{
		"Command": "echo", "Args": []string{"hello"}, "Reason": "greet",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hello") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestPermissionsCenterReflectsState(t *testing.T) {
	dir := t.TempDir()
	ag := agent.New(agent.Policy{WorkDir: dir, DenyPrefixes: []string{"curl"}}, nil)
	pc := &PermissionsCenter{Agent: ag, VisionEnabled: true}
	mux := http.NewServeMux()
	pc.RegisterPermissions(mux)

	rec := doJSON(t, mux, "GET", "/api/runyx/permissions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"enabled":true`) || !strings.Contains(body, dir) || !strings.Contains(body, "curl") {
		t.Errorf("body = %s", body)
	}
}

func TestApprovalsDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	a := &AgentExtras{}
	a.RegisterAgent(mux)

	rec := doJSON(t, mux, "GET", "/api/runyx/approvals", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET approvals status = %d, want 503 with no queue configured", rec.Code)
	}
	rec = doJSON(t, mux, "POST", "/api/runyx/approvals/x/decide", map[string]any{"Hash": "x", "Allow": true})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("decide status = %d, want 503 with no queue configured", rec.Code)
	}
}

func TestApprovalEndToEndOverHTTP(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	queue := approval.NewQueue()
	ag := agent.New(agent.Policy{WorkDir: dir}, func(ctx context.Context, req agent.Request) (bool, agent.ApprovalScope) {
		areq := approval.NewRequest("command", req.String(), req.String())
		d, err := queue.Submit(ctx, areq)
		if err != nil || !d.Allow {
			return false, agent.ApprovalDenied
		}
		return true, agent.ApprovalOnce
	})
	mux := http.NewServeMux()
	(&AgentExtras{Agent: ag, Approvals: queue}).RegisterAgent(mux)

	// Submit the command execution concurrently -- it blocks inside
	// the approval adapter until /approvals/{id}/decide resolves it,
	// same as a real HTTP client waiting on a slow handler.
	type execResult struct {
		rec *httptest.ResponseRecorder
	}
	resultCh := make(chan execResult, 1)
	go func() {
		rec := doJSON(t, mux, "POST", "/api/runyx/agent/execute", map[string]any{
			"Command": "echo", "Args": []string{"approved"}, "Reason": "greet",
		})
		resultCh <- execResult{rec: rec}
	}()

	// Poll until the request shows up as pending.
	var pending []approval.Request
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec := doJSON(t, mux, "GET", "/api/runyx/approvals", nil)
		json.Unmarshal(rec.Body.Bytes(), &pending)
		if len(pending) == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatalf("expected exactly one pending approval, got %d", len(pending))
	}

	decideRec := doJSON(t, mux, "POST", "/api/runyx/approvals/"+pending[0].ID+"/decide", map[string]any{
		"Hash": pending[0].Hash, "Allow": true,
	})
	if decideRec.Code != http.StatusOK {
		t.Fatalf("decide status = %d, body=%s", decideRec.Code, decideRec.Body.String())
	}

	select {
	case res := <-resultCh:
		if res.rec.Code != http.StatusOK {
			t.Fatalf("execute status = %d, body=%s", res.rec.Code, res.rec.Body.String())
		}
		if !strings.Contains(res.rec.Body.String(), "approved") {
			t.Errorf("execute body = %s", res.rec.Body.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("execute did not complete after the approval was granted")
	}
}

func TestApprovalDecideFailsOnHashMismatchOverHTTP(t *testing.T) {
	queue := approval.NewQueue()
	mux := http.NewServeMux()
	(&AgentExtras{Approvals: queue}).RegisterAgent(mux)

	req := approval.NewRequest("command", "rm -rf /", "rm -rf /")
	go queue.Submit(context.Background(), req)
	time.Sleep(10 * time.Millisecond)

	rec := doJSON(t, mux, "POST", "/api/runyx/approvals/"+req.ID+"/decide", map[string]any{
		"Hash": "wrong-hash", "Allow": true,
	})
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 on hash mismatch", rec.Code)
	}
}
