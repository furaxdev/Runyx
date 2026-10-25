// Package agent implements the safe shell agent (item 21): commands
// proposed by a model run only inside a designated working directory,
// only after approval, with every run diffed and logged, and undoable
// via git snapshots. This is the one place in Runyx that executes
// arbitrary commands, so every rule here is deliberately conservative
// and some are non-negotiable per the project's §7 security rules.
package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// ApprovalScope says how long an approval decision is remembered.
type ApprovalScope string

const (
	ApprovalOnce    ApprovalScope = "once"    // this single command only
	ApprovalSession ApprovalScope = "session" // any command matching the same reason, for this process's lifetime
	ApprovalAlways  ApprovalScope = "always"  // persisted; matching commands never ask again
	ApprovalDenied  ApprovalScope = "denied"  // user said no
)

// Request describes one command proposal awaiting approval.
type Request struct {
	Command string
	Args    []string
	WorkDir string
	Reason  string // why the model wants to run this, shown to the user
}

func (r Request) String() string {
	return strings.TrimSpace(r.Command + " " + strings.Join(r.Args, " "))
}

// ApprovalFunc is asked, for a command not already covered by a
// standing "always" approval, whether to run it and for how long to
// remember the answer. Implemented by the UI/CLI — this package never
// assumes consent. ctx is the same context Execute was called with: an
// implementation that blocks waiting for a human (e.g. an async
// approval queue) must respect ctx.Done() rather than waiting forever
// after the original request (e.g. the HTTP client) has gone away.
type ApprovalFunc func(ctx context.Context, req Request) (allow bool, scope ApprovalScope)

// Policy configures what the agent may ever consider running, before
// approval is even asked.
type Policy struct {
	// WorkDir is the only directory tree commands may touch. Required.
	WorkDir string
	// AllowPrefixes, if non-empty, restricts commands to those whose
	// first word (the executable) is in this list. Empty means "any
	// command not explicitly denied", i.e. DenyPrefixes alone decides.
	AllowPrefixes []string
	// DenyPrefixes blocks commands whose first word matches, regardless
	// of AllowPrefixes — e.g. "rm", "sudo", "curl" if not wanted.
	DenyPrefixes []string
}

// nonNegotiableDenyPrefixes can never be removed from a Policy by
// configuration: no sudo, ever, matching §7 ("no sudo... without
// explicit validation every time" — Runyx goes further and refuses it
// outright rather than offering an approval prompt for it).
var nonNegotiableDenyPrefixes = []string{"sudo", "doas", "su"}

// Validate checks a request against the policy and path-escape rules,
// before any approval is asked. Returns a non-nil error describing
// exactly why a command is refused.
func (p Policy) Validate(req Request) error {
	if p.WorkDir == "" {
		return fmt.Errorf("agent policy has no WorkDir configured")
	}
	absWorkDir, err := filepath.Abs(p.WorkDir)
	if err != nil {
		return fmt.Errorf("resolve work dir: %w", err)
	}

	exe := req.Command
	allDeny := append(append([]string{}, nonNegotiableDenyPrefixes...), p.DenyPrefixes...)
	if matchesDenied(exe, allDeny) {
		return fmt.Errorf("command %q is never allowed", exe)
	}
	if len(p.AllowPrefixes) > 0 && !contains(p.AllowPrefixes, exe) {
		return fmt.Errorf("command %q is not in the allowed list", exe)
	}

	// Refuse any argument that looks like an absolute path or a ".."
	// traversal escaping WorkDir — a conservative, string-level check;
	// real containment is enforced again at execution time by running
	// with Dir=WorkDir and (on POSIX) without a shell, so a refused
	// argument here is defense in depth, not the only gate.
	for _, a := range req.Args {
		if filepath.IsAbs(a) {
			resolved := filepath.Clean(a)
			if !strings.HasPrefix(resolved, absWorkDir) {
				return fmt.Errorf("argument %q points outside the working directory", a)
			}
		}
		if strings.Contains(a, "..") {
			joined := filepath.Clean(filepath.Join(absWorkDir, a))
			if !strings.HasPrefix(joined, absWorkDir) {
				return fmt.Errorf("argument %q would escape the working directory", a)
			}
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// matchesDenied reports whether exe names one of deny, checked against
// the *base name* of exe (case-insensitively), not its raw literal
// string. A plain `exe == deny` check -- what this replaces -- is
// trivially bypassed: "/bin/sudo", "./sudo", or on Windows "SUDO.EXE"
// all name the exact same program as "sudo" but would never match it
// literally, letting a categorically-refused command (no sudo, ever --
// see nonNegotiableDenyPrefixes) through to the approval prompt instead
// of being refused outright. filepath.Base handles both "/bin/sudo"
// and "./sudo"; the .exe/.bat/.cmd/.com suffix strip only ever changes
// behavior on an actual Windows PATHEXT-style executable name, so it
// can't cause a false match on POSIX (no such suffix there) or hide a
// real POSIX binary literally named e.g. "sudo.exe".
func matchesDenied(exe string, deny []string) bool {
	base := strings.ToLower(filepath.Base(exe))
	for _, ext := range []string{".exe", ".bat", ".cmd", ".com"} {
		base = strings.TrimSuffix(base, ext)
	}
	for _, d := range deny {
		if base == strings.ToLower(d) {
			return true
		}
	}
	return false
}
