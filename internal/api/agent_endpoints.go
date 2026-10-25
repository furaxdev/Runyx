package api

import (
	"encoding/json"
	"net/http"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/approval"
	"github.com/furaxdev/runyx/internal/interpreter"
)

// AgentExtras wires the safe shell agent (item 21) and the sandboxed
// code interpreter (item 22) into the HTTP API. Both are nil-safe: if
// not configured, their endpoints return 503 rather than panicking —
// a Runyx install with no agent configured still serves every other
// endpoint normally.
type AgentExtras struct {
	Agent       *agent.Agent
	Interpreter *interpreter.Interpreter
	// Approvals is the pending-approval queue backing Agent's
	// ApprovalFunc (see internal/approval and daemon.New's wiring).
	// Nil-safe: when nil, the approvals endpoints report 503 rather
	// than panicking, same pattern as Agent/Interpreter above.
	Approvals *approval.Queue
}

// RegisterAgent wires agent-shell, interpreter, and pending-approval
// routes onto mux.
func (a *AgentExtras) RegisterAgent(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/runyx/agent/execute", a.handleExecute)
	mux.HandleFunc("POST /api/runyx/agent/undo", a.handleUndo)
	mux.HandleFunc("POST /api/runyx/interpreter/run", a.handleInterpreterRun)
	mux.HandleFunc("GET /api/runyx/approvals", a.handleListApprovals)
	mux.HandleFunc("POST /api/runyx/approvals/{id}/decide", a.handleDecideApproval)
}

func (a *AgentExtras) handleExecute(w http.ResponseWriter, r *http.Request) {
	if a.Agent == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("the shell agent is not configured (set agent_work_dir in config.toml to enable it)"))
		return
	}
	var req struct {
		Command string
		Args    []string
		Reason  string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := a.Agent.Execute(r.Context(), req.Command, req.Args, req.Reason)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *AgentExtras) handleUndo(w http.ResponseWriter, r *http.Request) {
	if a.Agent == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("the shell agent is not configured (set agent_work_dir in config.toml to enable it)"))
		return
	}
	var req struct{ SnapshotID string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.Agent.Undo(r.Context(), req.SnapshotID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *AgentExtras) handleInterpreterRun(w http.ResponseWriter, r *http.Request) {
	if a.Interpreter == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("the code interpreter is not configured (set interpreter_work_dir in config.toml to enable it)"))
		return
	}
	var req struct{ Code string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := a.Interpreter.Run(r.Context(), req.Code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleListApprovals returns every pending approval request, for the
// TUI/web/paired-device UI to poll and show a human. Never includes
// anything already resolved or expired.
func (a *AgentExtras) handleListApprovals(w http.ResponseWriter, r *http.Request) {
	if a.Approvals == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("no approval queue is configured"))
		return
	}
	writeJSON(w, http.StatusOK, a.Approvals.List())
}

// handleDecideApproval answers one pending request. The hash in the
// body must match the request's current Hash exactly -- Resolve fails
// closed (never allows) on a mismatch, an unknown id, or an expired
// request, so this handler just relays whatever approval.Queue decides
// rather than second-guessing it.
func (a *AgentExtras) handleDecideApproval(w http.ResponseWriter, r *http.Request) {
	if a.Approvals == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("no approval queue is configured"))
		return
	}
	var req struct {
		Hash  string
		Allow bool
		Scope string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	scope := approval.Scope(req.Scope)
	if scope == "" {
		scope = approval.ScopeOnce
	}
	if !req.Allow {
		scope = approval.ScopeDenied
	}
	id := r.PathValue("id")
	if err := a.Approvals.Resolve(id, req.Hash, approval.Decision{Allow: req.Allow, Scope: scope}); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
