package contextmgmt

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("EstimateTokens(\"\") = %d, want 0", got)
	}
	if got := EstimateTokens("abcd"); got != 1 {
		t.Errorf("EstimateTokens(4 chars) = %d, want 1", got)
	}
	if got := EstimateTokens(strings.Repeat("a", 400)); got != 100 {
		t.Errorf("EstimateTokens(400 chars) = %d, want 100", got)
	}
}

func TestComputeGaugeTriggersCompressionPastThreshold(t *testing.T) {
	msgs := []Message{{Role: "user", Content: strings.Repeat("x", 4000)}}
	gauge := ComputeGauge(msgs, 1000) // ~1000 tokens used against a 1000 budget
	if !gauge.NeedsCompress {
		t.Error("expected NeedsCompress=true when usage exceeds 80% of budget")
	}
	if gauge.PercentUsed < 80 {
		t.Errorf("PercentUsed = %v, want >= 80", gauge.PercentUsed)
	}
}

func TestComputeGaugeNoCompressionWhenRoomy(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "short message"}}
	gauge := ComputeGauge(msgs, 100000)
	if gauge.NeedsCompress {
		t.Error("expected NeedsCompress=false with plenty of budget left")
	}
}

func TestCompressKeepsRecentAndSummarizesOlder(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "msg2"},
		{Role: "user", Content: "msg3"},
		{Role: "assistant", Content: "msg4"},
		{Role: "user", Content: "msg5 (recent)"},
	}
	var summarizedCount int
	summarize := func(ctx context.Context, old []Message) (string, error) {
		summarizedCount = len(old)
		return "summary of the first 4 messages", nil
	}

	compressed, err := Compress(context.Background(), msgs, 1, summarize)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if summarizedCount != 4 {
		t.Errorf("summarized %d messages, want 4", summarizedCount)
	}
	if len(compressed) != 2 {
		t.Fatalf("compressed = %d messages, want 2 (summary + 1 kept)", len(compressed))
	}
	if compressed[0].Role != "system" || !strings.Contains(compressed[0].Content, "summary") {
		t.Errorf("compressed[0] = %+v", compressed[0])
	}
	if compressed[1].Content != "msg5 (recent)" {
		t.Errorf("compressed[1] = %+v, want the most recent message preserved", compressed[1])
	}
}

func TestCompressNoOpWhenShortEnough(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}}
	compressed, err := Compress(context.Background(), msgs, 10, nil)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if len(compressed) != 1 {
		t.Errorf("expected no-op when under keepLast, got %v", compressed)
	}
}

func TestCompressPropagatesSummarizeError(t *testing.T) {
	msgs := []Message{{Content: "a"}, {Content: "b"}, {Content: "c"}}
	_, err := Compress(context.Background(), msgs, 1, func(ctx context.Context, m []Message) (string, error) {
		return "", errors.New("boom")
	})
	if err == nil {
		t.Error("expected error to propagate from summarize")
	}
}
