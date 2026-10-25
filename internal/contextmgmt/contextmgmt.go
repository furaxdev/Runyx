// Package contextmgmt implements long-context handling (item 19):
// estimating token usage for the visible "token gauge", and compressing
// old history into a summary once a conversation grows past a budget,
// instead of silently dropping or hard-truncating early messages.
package contextmgmt

import (
	"context"
	"fmt"
	"strings"
)

// Message mirrors the minimal shape contextmgmt needs, so this package
// doesn't import internal/history or internal/api (avoids a dependency
// cycle; callers adapt their own message type to this one).
type Message struct {
	Role    string
	Content string
}

// EstimateTokens approximates token count the same way internal/convert
// does for chunking (~4 characters per token), which is accurate enough
// for a progress gauge — exact counts need the model's own tokenizer,
// not available without loading it.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

// EstimateConversationTokens sums estimated tokens across every message,
// plus a small per-message overhead most chat templates add (role
// markers, separators).
func EstimateConversationTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateTokens(m.Content) + 4
	}
	return total
}

// Gauge is the token-usage summary shown in the UI.
type Gauge struct {
	UsedTokens    int
	BudgetTokens  int
	PercentUsed   float64
	NeedsCompress bool // true once usage crosses the compression threshold
}

// ComputeGauge reports how full the context window is. NeedsCompress
// becomes true above 80% of budget, leaving headroom for the reply
// itself and avoiding a hard failure mid-generation.
func ComputeGauge(msgs []Message, budgetTokens int) Gauge {
	used := EstimateConversationTokens(msgs)
	pct := 0.0
	if budgetTokens > 0 {
		pct = float64(used) / float64(budgetTokens) * 100
	}
	return Gauge{
		UsedTokens:    used,
		BudgetTokens:  budgetTokens,
		PercentUsed:   pct,
		NeedsCompress: budgetTokens > 0 && pct >= 80,
	}
}

// SummarizeFunc asks a model to summarize a slice of older messages into
// a short paragraph. Implemented by the caller (a call to
// /v1/chat/completions against a loaded model) so this package has no
// HTTP/backend dependency.
type SummarizeFunc func(ctx context.Context, msgs []Message) (string, error)

// Compress keeps the most recent keepLast messages verbatim and replaces
// everything older with one synthetic system message summarizing them,
// via summarize. If there's nothing old enough to summarize (len(msgs)
// <= keepLast), msgs is returned unchanged.
func Compress(ctx context.Context, msgs []Message, keepLast int, summarize SummarizeFunc) ([]Message, error) {
	if keepLast < 0 {
		keepLast = 0
	}
	if len(msgs) <= keepLast {
		return msgs, nil
	}
	cutoff := len(msgs) - keepLast
	older, recent := msgs[:cutoff], msgs[cutoff:]

	summary, err := summarize(ctx, older)
	if err != nil {
		return nil, fmt.Errorf("summarize older messages: %w", err)
	}

	compressed := make([]Message, 0, 1+len(recent))
	compressed = append(compressed, Message{
		Role:    "system",
		Content: "Summary of the earlier conversation (compressed to save context):\n" + strings.TrimSpace(summary),
	})
	compressed = append(compressed, recent...)
	return compressed, nil
}
