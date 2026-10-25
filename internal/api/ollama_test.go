package api

import (
	"strings"
	"testing"
)

func TestForEachSSELine(t *testing.T) {
	stream := "data: {\"a\":1}\n\ndata: {\"a\":2}\n\ndata: [DONE]\n\n"
	var seen []string
	forEachSSELine(strings.NewReader(stream), func(data string) { seen = append(seen, data) })
	if len(seen) != 2 {
		t.Fatalf("got %d lines, want 2 (DONE excluded): %v", len(seen), seen)
	}
}

func TestForEachSSEField(t *testing.T) {
	stream := "data: {\"content\":\"Hel\"}\n\ndata: {\"content\":\"lo\"}\n\n"
	var out strings.Builder
	forEachSSEField(strings.NewReader(stream), "content", func(v string) { out.WriteString(v) })
	if out.String() != "Hello" {
		t.Errorf("got %q, want %q", out.String(), "Hello")
	}
}
