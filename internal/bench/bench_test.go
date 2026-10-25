package bench

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeStreamingServer emits n token chunks, delay apart, mimicking
// llama-server's SSE streaming format closely enough to exercise Run's
// real parsing and real timing measurement.
func fakeStreamingServer(t *testing.T, n int, delay time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i := 0; i < n; i++ {
			time.Sleep(delay)
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"tok%d \"}}]}\n\n", i)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
}

func TestRunMeasuresRealTiming(t *testing.T) {
	srv := fakeStreamingServer(t, 10, 20*time.Millisecond)
	defer srv.Close()

	result, err := Run(context.Background(), srv.Client(), srv.URL, "test-model", "bonjour")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.TokenCount != 10 {
		t.Errorf("TokenCount = %d, want 10", result.TokenCount)
	}
	if result.TimeToFirstTok < 15*time.Millisecond {
		t.Errorf("TimeToFirstTok = %v, expected roughly >= 20ms", result.TimeToFirstTok)
	}
	if result.TotalDuration < 180*time.Millisecond {
		t.Errorf("TotalDuration = %v, expected roughly >= 200ms (10 * 20ms)", result.TotalDuration)
	}
	if result.TokensPerSec <= 0 {
		t.Errorf("TokensPerSec = %v, want > 0", result.TokensPerSec)
	}
	if result.Text == "" {
		t.Error("expected non-empty generated text")
	}
}

func TestRunHandlesErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := Run(context.Background(), srv.Client(), srv.URL, "m", "p")
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestRunArenaComparesMultipleTargets(t *testing.T) {
	srvA := fakeStreamingServer(t, 5, 5*time.Millisecond)
	defer srvA.Close()
	srvB := fakeStreamingServer(t, 5, 5*time.Millisecond)
	defer srvB.Close()

	arena, err := RunArena(context.Background(), srvA.Client(), "same prompt", []Target{
		{BaseURL: srvA.URL, Model: "model-a"},
		{BaseURL: srvB.URL, Model: "model-b"},
	})
	if err != nil {
		t.Fatalf("RunArena: %v", err)
	}
	if len(arena.Entries) != 2 {
		t.Fatalf("Entries = %v", arena.Entries)
	}
	if arena.Labels[0] != "A" || arena.Labels[1] != "B" {
		t.Errorf("Labels = %v, want [A B]", arena.Labels)
	}
	for _, e := range arena.Entries {
		if e.TokenCount != 5 {
			t.Errorf("entry %s: TokenCount = %d, want 5", e.Model, e.TokenCount)
		}
	}
}

func TestRunArenaPropagatesErrors(t *testing.T) {
	srvBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srvBad.Close()

	_, err := RunArena(context.Background(), srvBad.Client(), "p", []Target{{BaseURL: srvBad.URL, Model: "m"}})
	if err == nil {
		t.Error("expected error to propagate from a failing target")
	}
}
