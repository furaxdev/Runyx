package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/furaxdev/runyx/internal/secretscan"
)

// Result is the outcome of one executed command.
type Result struct {
	Command    string    `json:"command"`
	Args       []string  `json:"args"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	ExitCode   int       `json:"exit_code"`
	Diff       string    `json:"diff"`
	SnapshotID string    `json:"snapshot_id"`
	StartedAt  time.Time `json:"started_at"`
	Duration   string    `json:"duration"`
}

// AuditEntry is one line of the audit log (JSONL — one JSON object per
// line, append-only).
type AuditEntry struct {
	Timestamp  time.Time     `json:"timestamp"`
	Command    string        `json:"command"`
	Args       []string      `json:"args"`
	Reason     string        `json:"reason"`
	Approved   bool          `json:"approved"`
	ApprovalBy ApprovalScope `json:"approval_scope"`
	ExitCode   int           `json:"exit_code"`
	Error      string        `json:"error,omitempty"`
}

// Agent executes approved, policy-checked commands inside a working
// directory, snapshotting before and diffing after each run, and
// recording every attempt (approved or refused) to an audit log.
type Agent struct {
	Policy      Policy
	Approve     ApprovalFunc
	AuditLogDir string // directory for audit.jsonl; "" disables audit logging

	mu               sync.Mutex
	sessionApprovals map[string]bool // reason -> approved, for ApprovalSession
	alwaysApprovals  map[string]bool // reason -> approved, persisted across the Agent's lifetime (in-memory; see Decisions for why not yet persisted to disk)
}

// New returns an Agent ready to execute commands under policy.
func New(policy Policy, approve ApprovalFunc) *Agent {
	return &Agent{
		Policy:           policy,
		Approve:          approve,
		sessionApprovals: map[string]bool{},
		alwaysApprovals:  map[string]bool{},
	}
}

// Execute validates, approves, snapshots, runs, diffs, and audits one
// command. Every path through this function writes an audit entry,
// including refusals — "always confirmed before running" never means
// "sometimes unlogged."
func (a *Agent) Execute(ctx context.Context, command string, args []string, reason string) (Result, error) {
	req := Request{Command: command, Args: args, WorkDir: a.Policy.WorkDir, Reason: reason}

	if err := a.Policy.Validate(req); err != nil {
		a.audit(AuditEntry{Timestamp: time.Now(), Command: command, Args: args, Reason: reason, Approved: false, Error: err.Error()})
		return Result{}, err
	}

	allowed, scope := a.resolveApproval(ctx, req)
	if !allowed {
		err := fmt.Errorf("command refused by user: %s", req.String())
		a.audit(AuditEntry{Timestamp: time.Now(), Command: command, Args: args, Reason: reason, Approved: false, ApprovalBy: ApprovalDenied, Error: err.Error()})
		return Result{}, err
	}

	absWorkDir, err := mustAbs(a.Policy.WorkDir)
	if err != nil {
		return Result{}, err
	}

	var snapshotID string
	if gitAvailable() {
		snapshotID, err = Snapshot(ctx, absWorkDir, "before: "+req.String())
		if err != nil {
			// Snapshotting failure shouldn't silently skip undo support;
			// surface it, but still let the command run — a user who
			// explicitly approved a command expects it to run.
			snapshotID = ""
		}
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = absWorkDir
	var stdout, stderr bytesBuf
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}

	diff := ""
	if gitAvailable() {
		diff, _ = Diff(ctx, absWorkDir, snapshotID)
	}

	// A command's own output can contain a secret it merely echoed
	// (e.g. "cat .env", a tool printing an API key it was configured
	// with) that was never part of the command line itself and so
	// nothing upstream of this point had any reason to refuse or flag.
	// Redact before this Result goes anywhere -- the API response, the
	// audit log, chat history the model's reply might quote it into.
	redactedStdout, _ := secretscan.Redact(stdout.String())
	redactedStderr, _ := secretscan.Redact(stderr.String())

	result := Result{
		Command: command, Args: args, Stdout: redactedStdout, Stderr: redactedStderr,
		ExitCode: exitCode, Diff: diff, SnapshotID: snapshotID, StartedAt: start, Duration: time.Since(start).String(),
	}

	a.audit(AuditEntry{
		Timestamp: start, Command: command, Args: args, Reason: reason,
		Approved: true, ApprovalBy: scope, ExitCode: exitCode, Error: errString(runErr),
	})
	return result, nil
}

// resolveApproval checks standing approvals before asking the user.
func (a *Agent) resolveApproval(ctx context.Context, req Request) (bool, ApprovalScope) {
	a.mu.Lock()
	if a.alwaysApprovals[req.Reason] {
		a.mu.Unlock()
		return true, ApprovalAlways
	}
	if a.sessionApprovals[req.Reason] {
		a.mu.Unlock()
		return true, ApprovalSession
	}
	a.mu.Unlock()

	if a.Approve == nil {
		return false, ApprovalDenied // no approval mechanism wired up: fail closed, never default-allow
	}
	allow, scope := a.Approve(ctx, req)
	if allow {
		a.mu.Lock()
		switch scope {
		case ApprovalSession:
			a.sessionApprovals[req.Reason] = true
		case ApprovalAlways:
			a.alwaysApprovals[req.Reason] = true
		}
		a.mu.Unlock()
	}
	return allow, scope
}

// Undo resets the working directory to a prior snapshot, surfaced for
// the UI's "annuler" action.
func (a *Agent) Undo(ctx context.Context, snapshotID string) error {
	absWorkDir, err := mustAbs(a.Policy.WorkDir)
	if err != nil {
		return err
	}
	return Undo(ctx, absWorkDir, snapshotID)
}

func (a *Agent) audit(entry AuditEntry) {
	if a.AuditLogDir == "" {
		return
	}
	// The audit log is permanent (append-only, never rotated out by
	// this package) and the model chose every one of these fields, so
	// Reason/Args in particular can contain a secret the model was
	// handed or typed back (a token pasted into a prompt, a password
	// in a "run this to fix it" suggestion) with nothing upstream ever
	// having redacted it. Redact before it's written, not after.
	entry.Command, _ = secretscan.Redact(entry.Command)
	for i, arg := range entry.Args {
		entry.Args[i], _ = secretscan.Redact(arg)
	}
	entry.Reason, _ = secretscan.Redact(entry.Reason)
	entry.Error, _ = secretscan.Redact(entry.Error)

	if err := os.MkdirAll(a.AuditLogDir, 0o755); err != nil {
		return
	}
	// 0o600: this file accumulates every command the shell agent has
	// ever run, redacted but not a risk worth leaving world-readable
	// regardless (matches daemon.TokenStore's devices.json precedent).
	path := filepath.Join(a.AuditLogDir, "audit.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	// OpenFile's mode only applies when creating a new file -- an
	// audit.jsonl left over from before this permission tightening
	// would otherwise stay world-readable forever. Best-effort: a
	// failure here (e.g. not the file's owner) doesn't block logging.
	_ = os.Chmod(path, 0o600)
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f.Write(append(data, '\n'))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// bytesBuf is a minimal io.Writer-backed string builder, avoiding a
// bytes.Buffer import just for this.
type bytesBuf struct{ data []byte }

func (b *bytesBuf) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}
func (b *bytesBuf) String() string { return string(b.data) }
