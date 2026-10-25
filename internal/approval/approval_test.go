package approval

import (
	"context"
	"testing"
	"time"
)

func TestSubmitBlocksUntilResolve(t *testing.T) {
	q := NewQueue()
	req := NewRequest("command", "rm -rf build/", "rm -rf build/")

	resultCh := make(chan Decision, 1)
	errCh := make(chan error, 1)
	go func() {
		d, err := q.Submit(context.Background(), req)
		resultCh <- d
		errCh <- err
	}()

	// Give Submit a moment to register before resolving.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(q.List()) == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(q.List()) != 1 {
		t.Fatal("expected the request to appear in List() before being resolved")
	}

	if err := q.Resolve(req.ID, req.Hash, Decision{Allow: true, Scope: ScopeOnce}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	d := <-resultCh
	if err := <-errCh; err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if !d.Allow || d.Scope != ScopeOnce {
		t.Errorf("Decision = %+v, want {Allow:true Scope:once}", d)
	}
	if len(q.List()) != 0 {
		t.Error("expected the request to be removed from List() after resolution")
	}
}

func TestResolveFailsOnHashMismatch(t *testing.T) {
	q := NewQueue()
	req := NewRequest("command", "echo hi", "echo hi")
	go q.Submit(context.Background(), req)

	time.Sleep(10 * time.Millisecond)
	err := q.Resolve(req.ID, "not-the-real-hash", Decision{Allow: true, Scope: ScopeOnce})
	if err == nil {
		t.Fatal("expected Resolve to fail when the hash does not match")
	}
}

func TestResolveFailsOnUnknownID(t *testing.T) {
	q := NewQueue()
	if err := q.Resolve("does-not-exist", "anything", Decision{Allow: true, Scope: ScopeOnce}); err == nil {
		t.Fatal("expected Resolve to fail for an unknown request id")
	}
}

func TestExpiredRequestIsDeniedNotLeftHanging(t *testing.T) {
	q := NewQueue()
	frozen := time.Now()
	q.setClock(func() time.Time { return frozen })

	req := NewRequest("command", "echo hi", "echo hi")
	// Simulate the clock advancing past the request's own expiry.
	q.setClock(func() time.Time { return frozen.Add(DefaultTTL + time.Second) })

	d, err := q.Submit(context.Background(), req)
	if err == nil {
		t.Fatal("expected Submit to report an error for an already-expired request")
	}
	if d.Allow {
		t.Error("an expired request must never resolve as allowed")
	}
}

func TestExpiredRequestIsExcludedFromList(t *testing.T) {
	q := NewQueue()
	frozen := time.Now()
	q.setClock(func() time.Time { return frozen })
	req := NewRequest("command", "echo hi", "echo hi")

	done := make(chan struct{})
	go func() {
		q.Submit(context.Background(), req)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)

	q.setClock(func() time.Time { return frozen.Add(DefaultTTL + time.Second) })

	if got := q.List(); len(got) != 0 {
		t.Errorf("List() = %v, want empty once the request is past its expiry", got)
	}
}

func TestResolveFailsAfterExpiry(t *testing.T) {
	q := NewQueue()
	frozen := time.Now()
	q.setClock(func() time.Time { return frozen })
	req := NewRequest("command", "echo hi", "echo hi")
	q.mu.Lock()
	q.pending[req.ID] = &pendingEntry{req: req, resultCh: make(chan Decision, 1)}
	q.mu.Unlock()

	q.setClock(func() time.Time { return frozen.Add(DefaultTTL + time.Second) })
	if err := q.Resolve(req.ID, req.Hash, Decision{Allow: true, Scope: ScopeOnce}); err == nil {
		t.Fatal("expected Resolve to fail for an expired request even with a correct hash")
	}
}

func TestContextCancelDeniesRatherThanHangs(t *testing.T) {
	q := NewQueue()
	req := NewRequest("command", "echo hi", "echo hi")
	ctx, cancel := context.WithCancel(context.Background())

	resultCh := make(chan Decision, 1)
	go func() {
		d, _ := q.Submit(ctx, req)
		resultCh <- d
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case d := <-resultCh:
		if d.Allow {
			t.Error("a cancelled wait must never resolve as allowed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Submit did not return promptly after context cancellation")
	}
}

func TestResolveOnlyOnce(t *testing.T) {
	q := NewQueue()
	req := NewRequest("command", "echo hi", "echo hi")
	go q.Submit(context.Background(), req)
	time.Sleep(10 * time.Millisecond)

	if err := q.Resolve(req.ID, req.Hash, Decision{Allow: true, Scope: ScopeOnce}); err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	// The entry is removed by Submit's own deferred cleanup shortly
	// after resolution; a second Resolve must not succeed either way.
	time.Sleep(10 * time.Millisecond)
	if err := q.Resolve(req.ID, req.Hash, Decision{Allow: true, Scope: ScopeOnce}); err == nil {
		t.Fatal("expected a second Resolve for the same request to fail")
	}
}

func TestHashOfBindsToExactContent(t *testing.T) {
	a := HashOf("command", "rm file.txt")
	b := HashOf("command", "rm file.txt ")
	if a == b {
		t.Error("HashOf must distinguish even a trailing-space difference in detail")
	}
	c := HashOf("write_file", "rm file.txt")
	if a == c {
		t.Error("HashOf must distinguish by kind too, not just detail")
	}
}
