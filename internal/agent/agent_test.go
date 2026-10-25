package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in this environment")
	}
}

func TestExecuteRunsApprovedCommandAndRecordsDiff(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()

	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) { return true, ApprovalOnce })
	a.AuditLogDir = filepath.Join(dir, ".runyx-audit")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := a.Execute(ctx, "sh", []string{"-c", "echo hello > newfile.txt"}, "create a test file")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, stderr=%q", result.ExitCode, result.Stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "newfile.txt")); err != nil {
		t.Fatal("expected newfile.txt to exist after the command ran")
	}
	if !strings.Contains(result.Diff, "newfile.txt") {
		t.Errorf("Diff = %q, expected it to mention newfile.txt", result.Diff)
	}
	if result.SnapshotID == "" {
		t.Error("expected a snapshot ID to be recorded")
	}

	auditData, err := os.ReadFile(filepath.Join(a.AuditLogDir, "audit.jsonl"))
	if err != nil || len(auditData) == 0 {
		t.Fatalf("expected a non-empty audit log, err=%v", err)
	}
	if !strings.Contains(string(auditData), "newfile.txt") {
		t.Errorf("audit log missing command args: %s", auditData)
	}
}

func TestExecuteRefusesWithoutApproval(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) { return false, ApprovalDenied })

	_, err := a.Execute(context.Background(), "sh", []string{"-c", "echo nope > f.txt"}, "test")
	if err == nil {
		t.Fatal("expected refusal error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "f.txt")); statErr == nil {
		t.Error("command must not have run when approval was denied")
	}
}

func TestExecuteNeverAllowsSudo(t *testing.T) {
	dir := t.TempDir()
	calledApprove := false
	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) {
		calledApprove = true
		return true, ApprovalOnce
	})
	_, err := a.Execute(context.Background(), "sudo", []string{"rm", "-rf", "/"}, "anything")
	if err == nil {
		t.Fatal("expected sudo to be refused")
	}
	if calledApprove {
		t.Error("sudo must be refused before ever asking for approval")
	}
}

func TestExecuteRefusesPathEscape(t *testing.T) {
	dir := t.TempDir()
	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) { return true, ApprovalOnce })
	_, err := a.Execute(context.Background(), "cat", []string{"../../../etc/passwd"}, "read a file")
	if err == nil {
		t.Fatal("expected a path-escape refusal")
	}
}

func TestExecuteRespectsDenyPrefixes(t *testing.T) {
	dir := t.TempDir()
	a := New(Policy{WorkDir: dir, DenyPrefixes: []string{"curl", "wget"}}, func(context.Context, Request) (bool, ApprovalScope) { return true, ApprovalOnce })
	_, err := a.Execute(context.Background(), "curl", []string{"http://example.com"}, "fetch")
	if err == nil {
		t.Fatal("expected curl to be denied by policy")
	}
}

func TestSessionApprovalRemembersForSameReason(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	calls := 0
	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) {
		calls++
		return true, ApprovalSession
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	a.Execute(ctx, "sh", []string{"-c", "echo a > a.txt"}, "write files")
	a.Execute(ctx, "sh", []string{"-c", "echo b > b.txt"}, "write files")

	if calls != 1 {
		t.Errorf("expected the approval callback to run once (session-scoped), got %d calls", calls)
	}
}

func TestUndoRevertsToSnapshot(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) { return true, ApprovalOnce })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := a.Execute(ctx, "sh", []string{"-c", "echo original > tracked.txt"}, "write"); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// The snapshot taken immediately before the *second* command
	// represents the state right after the first one — i.e. the
	// "original" content — which is what we want to undo back to.
	result2, err := a.Execute(ctx, "sh", []string{"-c", "echo modified > tracked.txt"}, "overwrite")
	if err != nil {
		t.Fatalf("Execute (overwrite): %v", err)
	}
	snapshot := result2.SnapshotID
	if snapshot == "" {
		t.Skip("no git snapshot was recorded (git may be unavailable)")
	}

	content, _ := os.ReadFile(filepath.Join(dir, "tracked.txt"))
	if strings.TrimSpace(string(content)) != "modified" {
		t.Fatalf("expected file to say 'modified' before undo, got %q", content)
	}

	if err := a.Undo(ctx, snapshot); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	content, _ = os.ReadFile(filepath.Join(dir, "tracked.txt"))
	if strings.TrimSpace(string(content)) != "original" {
		t.Errorf("after Undo, expected 'original', got %q", content)
	}
}

func TestPolicyValidateRejectsMissingWorkDir(t *testing.T) {
	p := Policy{}
	if err := p.Validate(Request{Command: "echo"}); err == nil {
		t.Error("expected error when WorkDir is empty")
	}
}

func TestNoApprovalFuncFailsClosed(t *testing.T) {
	dir := t.TempDir()
	a := New(Policy{WorkDir: dir}, nil)
	_, err := a.Execute(context.Background(), "echo", []string{"hi"}, "test")
	if err == nil {
		t.Error("expected failure when no approval function is configured (fail closed)")
	}
}

// fakeSecret is a known-fake, test-only credential shaped like a real
// one (generic "KEY=value" form secretscan's "credential" pattern
// matches) -- never a real token. Allowlisted in .gitleaks.toml.
const fakeSecret = "API_KEY=sk-fake1234567890abcdefghij"

func TestExecuteRedactsSecretsInCommandOutput(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) { return true, ApprovalOnce })

	result, err := a.Execute(context.Background(), "echo", []string{fakeSecret}, "echo a secret")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(result.Stdout, "sk-fake1234567890abcdefghij") {
		t.Errorf("Stdout still contains the raw secret: %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "[redacted:") {
		t.Errorf("Stdout = %q, expected a [redacted:...] marker", result.Stdout)
	}
}

func TestAuditLogRedactsSecretsInReasonAndArgs(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) { return true, ApprovalOnce })
	a.AuditLogDir = filepath.Join(dir, ".runyx-audit")

	_, err := a.Execute(context.Background(), "echo", []string{fakeSecret}, "reason mentions "+fakeSecret)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	auditData, err := os.ReadFile(filepath.Join(a.AuditLogDir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if strings.Contains(string(auditData), "sk-fake1234567890abcdefghij") {
		t.Errorf("audit log still contains the raw secret: %s", auditData)
	}
	if !strings.Contains(string(auditData), "[redacted:") {
		t.Errorf("audit log = %s, expected a [redacted:...] marker", auditData)
	}
}

func TestAuditLogFileIsNotWorldReadable(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	a := New(Policy{WorkDir: dir}, func(context.Context, Request) (bool, ApprovalScope) { return true, ApprovalOnce })
	a.AuditLogDir = filepath.Join(dir, ".runyx-audit")

	if _, err := a.Execute(context.Background(), "echo", []string{"hi"}, "test"); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	info, err := os.Stat(filepath.Join(a.AuditLogDir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("stat audit log: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("audit.jsonl mode = %v, expected no group/other permission bits", info.Mode().Perm())
	}
}
