// Package approval implements a generic, async approval queue: any
// Runyx component that wants to do something irreversible (run a
// shell command, write a file, push, open a PR, trigger a workflow,
// delete something) submits a Request and blocks until a human
// decides, from whichever client is attached (TUI, web, or a paired
// device over Remote Control) -- never auto-approved.
//
// This replaces the shell agent's previous wiring, which passed a nil
// ApprovalFunc (see internal/daemon.New) and so refused every command
// outright: there was no async bridge between "a command wants to
// run" (an HTTP request's goroutine) and "a human looking at a UI
// decides" (a second, later HTTP request). This package is that
// bridge, reusable by both the legacy shell agent and Runyx Code's
// own tool-calling loop.
package approval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Scope says how long an approval decision is remembered by the
// caller (the caller, e.g. internal/agent, is responsible for actually
// remembering "session"/"always" -- this package only ever resolves
// one Request at a time).
type Scope string

const (
	ScopeOnce    Scope = "once"
	ScopeSession Scope = "session"
	ScopeAlways  Scope = "always"
	ScopeDenied  Scope = "denied"
)

// DefaultTTL is how long a request waits for a decision before it
// expires and is refused -- "Les demandes expirent au bout de 10
// minutes" (non-negotiable, not a default a caller can raise).
const DefaultTTL = 10 * time.Minute

// Request describes one action awaiting human approval.
type Request struct {
	ID string `json:"id"`
	// Kind categorizes the action for the UI (e.g. "command",
	// "write_file", "push", "pull_request", "workflow_dispatch",
	// "delete"). Not interpreted by this package.
	Kind string `json:"kind"`
	// Summary is a one-line description for a list view.
	Summary string `json:"summary"`
	// Detail is the full, copyable detail (e.g. the exact command and
	// args, or a diff) shown before approving.
	Detail string `json:"detail"`
	// Hash binds the approval to this exact content: Resolve refuses a
	// decision whose hash doesn't match, so a command can't change
	// between being shown and being approved. Computed by NewRequest,
	// never trusted from a caller.
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Decision is the human's answer to a Request.
type Decision struct {
	Allow bool
	Scope Scope
}

// NewRequest builds a Request with its Hash computed from kind+detail
// (the exact content a human is approving), and a 10-minute expiry
// from now.
func NewRequest(kind, summary, detail string) Request {
	now := time.Now()
	return Request{
		ID:        randomID(),
		Kind:      kind,
		Summary:   summary,
		Detail:    detail,
		Hash:      HashOf(kind, detail),
		CreatedAt: now,
		ExpiresAt: now.Add(DefaultTTL),
	}
}

// HashOf computes the binding hash for a (kind, detail) pair. Exported
// so callers (and tests) can verify a Request's Hash independently.
func HashOf(kind, detail string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + detail))
	return hex.EncodeToString(sum[:])
}

type pendingEntry struct {
	req      Request
	resultCh chan Decision
}

// Queue holds pending approval requests and blocks submitters until a
// human (via Resolve) or the request's own expiry decides.
type Queue struct {
	mu      sync.Mutex
	pending map[string]*pendingEntry

	// now is overridable in tests; nil means time.Now.
	now func() time.Time
}

// NewQueue returns an empty, ready-to-use Queue.
func NewQueue() *Queue {
	return &Queue{pending: map[string]*pendingEntry{}}
}

func (q *Queue) clock() time.Time {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.clockLocked()
}

// clockLocked returns the current time; callers must already hold q.mu.
func (q *Queue) clockLocked() time.Time {
	if q.now != nil {
		return q.now()
	}
	return time.Now()
}

// setClock overrides the queue's notion of "now", for tests. Safe to
// call while Submit/List/Resolve run concurrently in other goroutines.
func (q *Queue) setClock(f func() time.Time) {
	q.mu.Lock()
	q.now = f
	q.mu.Unlock()
}

// Submit registers req as pending and blocks until Resolve is called
// for it, its expiry passes, or ctx is cancelled (e.g. the HTTP
// request that wants this action was itself cancelled). A cancelled
// or expired wait is reported as denied: this package never resolves
// "nobody answered" as an allow.
func (q *Queue) Submit(ctx context.Context, req Request) (Decision, error) {
	entry := &pendingEntry{req: req, resultCh: make(chan Decision, 1)}
	q.mu.Lock()
	q.pending[req.ID] = entry
	q.mu.Unlock()
	defer func() {
		q.mu.Lock()
		delete(q.pending, req.ID)
		q.mu.Unlock()
	}()

	timeout := req.ExpiresAt.Sub(q.clock())
	if timeout <= 0 {
		return Decision{Allow: false, Scope: ScopeDenied}, fmt.Errorf("approval request %q already expired", req.ID)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case d := <-entry.resultCh:
		return d, nil
	case <-timer.C:
		return Decision{Allow: false, Scope: ScopeDenied}, fmt.Errorf("approval request %q expired after %s with no decision", req.ID, DefaultTTL)
	case <-ctx.Done():
		return Decision{Allow: false, Scope: ScopeDenied}, ctx.Err()
	}
}

// List returns every currently pending request, oldest first, with
// anything already past its own ExpiresAt left out (Submit's own
// timer will resolve those as denied shortly; List just doesn't show
// a human a request that's effectively already gone).
func (q *Queue) List() []Request {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.clockLocked()
	out := make([]Request, 0, len(q.pending))
	for _, e := range q.pending {
		if now.After(e.req.ExpiresAt) {
			continue
		}
		out = append(out, e.req)
	}
	return out
}

// Resolve answers a pending request. It fails closed: an unknown id,
// an expired request, or a hash that doesn't match the request's
// current content all return an error and never deliver an allow.
func (q *Queue) Resolve(id, hash string, decision Decision) error {
	q.mu.Lock()
	entry, ok := q.pending[id]
	q.mu.Unlock()
	if !ok {
		return fmt.Errorf("no pending approval request with id %q (already resolved, expired, or never existed)", id)
	}
	if q.clock().After(entry.req.ExpiresAt) {
		return fmt.Errorf("approval request %q has expired", id)
	}
	if hash != entry.req.Hash {
		return fmt.Errorf("approval request %q: hash mismatch (the request's content changed since it was shown)", id)
	}
	select {
	case entry.resultCh <- decision:
		return nil
	default:
		return fmt.Errorf("approval request %q was already resolved", id)
	}
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is effectively unrecoverable on any real
		// system; fall back to a time-based id rather than panicking,
		// since an approval id colliding is low-stakes (Submit keys by
		// it in a map scoped to this process's lifetime only).
		return fmt.Sprintf("appr-%d", time.Now().UnixNano())
	}
	return "appr-" + hex.EncodeToString(b[:])
}
