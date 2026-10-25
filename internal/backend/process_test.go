package backend

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/furaxdev/runyx/internal/modelstore"
)

// TestServerArgsPassesFlashAttnAValue guards a real CI failure: a
// current llama-server build rejects a bare "--flash-attn" flag
// ("expected value for argument") because the flag now takes an
// explicit on/off/auto value. Found via internal/backend's new output
// capture in a real CI run after llama-server had started crash-free
// for the first time (past the earlier release-resolution and
// extraction bugs).
func TestServerArgsPassesFlashAttnAValue(t *testing.T) {
	args := serverArgs("/models/foo.gguf", 12345, modelstore.TuningParams{
		CtxSize: 4096, BatchSize: 512, Threads: 4, FlashAttn: true,
	})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--flash-attn on") {
		t.Errorf("args = %q, want \"--flash-attn on\" (a bare flag is now a parse error)", joined)
	}
}

// TestServerArgsOmitsFlashAttnWhenNotTuned keeps the previous
// behavior for a model tuned without flash attention: no flag at all
// (llama-server defaults to "auto").
func TestServerArgsOmitsFlashAttnWhenNotTuned(t *testing.T) {
	args := serverArgs("/models/foo.gguf", 12345, modelstore.TuningParams{
		CtxSize: 4096, BatchSize: 512, Threads: 4, FlashAttn: false,
	})
	for _, a := range args {
		if a == "--flash-attn" {
			t.Errorf("args = %v, did not expect --flash-attn when tuning.FlashAttn is false", args)
		}
	}
}

// TestServerArgsAlwaysPassesJinja guards the actual root cause of the
// "coherent as raw /completion and as /v1/chat/completions run by
// hand, garbled as Runyx" bug report: without --jinja, llama-server
// renders chat messages with its own built-in guess at a template
// instead of the model's real one from the GGUF, which is wrong for
// some models (Qwen2.5 was the one that surfaced it) and produces
// unreadable output even though the binary, the GGUF and the machine
// are all fine.
func TestServerArgsAlwaysPassesJinja(t *testing.T) {
	args := serverArgs("/models/foo.gguf", 12345, modelstore.TuningParams{
		CtxSize: 4096, BatchSize: 512, Threads: 4,
	})
	found := false
	for _, a := range args {
		if a == "--jinja" {
			found = true
		}
	}
	if !found {
		t.Errorf("args = %v, want --jinja so llama-server renders the model's own chat template instead of guessing one", args)
	}
}

// TestWaitHealthyFailsFastOnEarlyExit guards the other half of the
// libgomp bug report: before this, a subprocess that died instantly
// (missing shared library, bad flag) still made waitHealthy poll an
// unreachable /health endpoint for the full 30-second deadline. Using
// an unroutable address means the HTTP GET itself won't resolve
// instantly either, so a loose bound (well under 30s) is enough to
// prove this returns on the exited signal rather than the timeout.
func TestWaitHealthyFailsFastOnEarlyExit(t *testing.T) {
	exited := make(chan error, 1)
	wantErr := errors.New("exit status 127")
	exited <- wantErr

	start := time.Now()
	err := waitHealthy(context.Background(), "http://127.0.0.1:1", exited)
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "exit status 127") {
		t.Fatalf("err = %v, want it to wrap the process exit error", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("waitHealthy took %s to notice the process had already exited, want well under the 30s timeout", elapsed)
	}
}

// TestDiagnoseStartupFailureLibgomp guards the actionable-message half
// of the libgomp bug report: the raw dynamic-linker error buried in
// captured stderr should turn into an instruction a non-C++ user can
// actually follow ("install libgomp1"), not just get echoed back.
func TestDiagnoseStartupFailureLibgomp(t *testing.T) {
	raw := "llama-server: error while loading shared libraries: libgomp.so.1: cannot open shared object file: No such file or directory"
	got := diagnoseStartupFailure(raw)
	if !strings.Contains(got, "libgomp1") && !strings.Contains(got, "libgomp") {
		t.Errorf("diagnoseStartupFailure(%q) = %q, want it to name the libgomp package", raw, got)
	}
	if !strings.Contains(got, raw) {
		t.Errorf("diagnoseStartupFailure(%q) = %q, want the raw output preserved too", raw, got)
	}
}

// TestDiagnoseStartupFailureGenericPassesThroughUnrecognizedOutput
// guards against diagnoseStartupFailure ever silently dropping output
// it doesn't recognize a pattern in -- an unknown failure should still
// show the raw llama-server output, just without a specific hint.
func TestDiagnoseStartupFailureGenericPassesThroughUnrecognizedOutput(t *testing.T) {
	raw := "llama-server: some unrelated crash nobody has seen before"
	got := diagnoseStartupFailure(raw)
	if !strings.Contains(got, raw) {
		t.Errorf("diagnoseStartupFailure(%q) = %q, want the raw output preserved", raw, got)
	}
}

// TestRingBufferKeepsOnlyTheTail guards the buffer that captures a
// started llama-server's stdout/stderr for diagnosing startup failures:
// it must cap at maxSize and keep the most recent bytes, not the first
// ones (the useful error is usually near the end of the output).
func TestRingBufferKeepsOnlyTheTail(t *testing.T) {
	rb := newRingBuffer(10)
	rb.Write([]byte("0123456789"))
	rb.Write([]byte("abcde"))
	got := rb.String()
	if len(got) != 10 {
		t.Fatalf("len = %d, want 10", len(got))
	}
	if !strings.HasSuffix("0123456789abcde", got) {
		t.Errorf("String() = %q, want the tail of the written bytes", got)
	}
}

func TestLRUVictimPicksLeastRecentlyUsed(t *testing.T) {
	now := time.Now()
	a := &Instance{ModelName: "a", lastUsed: now.Add(-10 * time.Minute)}
	b := &Instance{ModelName: "b", lastUsed: now.Add(-1 * time.Minute)}
	c := &Instance{ModelName: "c", lastUsed: now}
	victim := lruVictim([]*Instance{b, a, c})
	if victim.ModelName != "a" {
		t.Errorf("victim = %q, want %q (oldest lastUsed)", victim.ModelName, "a")
	}
}

func TestLRUVictimNilOnEmpty(t *testing.T) {
	if v := lruVictim(nil); v != nil {
		t.Errorf("expected nil victim for empty instance list, got %v", v)
	}
}

func TestMakeRoomLockedNoOpWhenUnlimited(t *testing.T) {
	m := NewManager("llama-server", time.Minute, 0)
	if err := m.makeRoomLocked(999999); err != nil {
		t.Errorf("expected no error with unlimited VRAM budget, got %v", err)
	}
}

func TestMakeRoomLockedErrorsWhenNothingToEvict(t *testing.T) {
	m := NewManager("llama-server", time.Minute, 1000)
	// No instances running, and 2000MB doesn't fit in a 1000MB budget.
	if err := m.makeRoomLocked(2000); err == nil {
		t.Error("expected error when model exceeds budget with nothing to evict")
	}
}

func TestInstanceTouchResetsIdle(t *testing.T) {
	inst := &Instance{lastUsed: time.Now().Add(-time.Hour)}
	if inst.IdleFor() < 59*time.Minute {
		t.Fatal("expected instance to appear idle before Touch")
	}
	inst.Touch()
	if inst.IdleFor() > time.Second {
		t.Error("expected IdleFor to be near zero right after Touch")
	}
}
