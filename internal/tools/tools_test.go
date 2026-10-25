package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRegisterAndDispatch(t *testing.T) {
	r := NewRegistry()
	r.Register(FunctionSpec{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"city": map[string]any{"type": "string"}},
			"required":   []string{"city"},
		},
	}, func(ctx context.Context, rawArgs json.RawMessage) (string, error) {
		var args struct{ City string }
		json.Unmarshal(rawArgs, &args)
		return "sunny in " + args.City, nil
	})

	if len(r.Defs()) != 1 || r.Defs()[0].Function.Name != "get_weather" {
		t.Fatalf("Defs() = %v", r.Defs())
	}

	calls := []ToolCall{{ID: "call_1", Function: struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}{Name: "get_weather", Arguments: `{"city":"Paris"}`}}}

	results := r.Dispatch(context.Background(), calls)
	if len(results) != 1 {
		t.Fatalf("results = %v", results)
	}
	if results[0].ToolCallID != "call_1" {
		t.Errorf("ToolCallID = %q", results[0].ToolCallID)
	}
	if !strings.Contains(results[0].Content, "Paris") {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestDispatchUnknownToolReturnsError(t *testing.T) {
	r := NewRegistry()
	calls := []ToolCall{{ID: "x", Function: struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}{Name: "does_not_exist", Arguments: "{}"}}}
	results := r.Dispatch(context.Background(), calls)
	if !strings.Contains(results[0].Content, "unknown tool") {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestRegisterReplacesExisting(t *testing.T) {
	r := NewRegistry()
	r.Register(FunctionSpec{Name: "x"}, func(ctx context.Context, a json.RawMessage) (string, error) { return "v1", nil })
	r.Register(FunctionSpec{Name: "x"}, func(ctx context.Context, a json.RawMessage) (string, error) { return "v2", nil })
	if len(r.Defs()) != 1 {
		t.Fatalf("expected re-registering to replace, got %d defs", len(r.Defs()))
	}
	results := r.Dispatch(context.Background(), []ToolCall{{Function: struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}{Name: "x"}}})
	if results[0].Content != "v2" {
		t.Errorf("Content = %q, want v2 (the replacement)", results[0].Content)
	}
}

func TestExtractToolCalls(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"tool_calls":[{"id":"c1","type":"function","function":{"name":"foo","arguments":"{}"}}]}}]}`)
	calls, err := ExtractToolCalls(body)
	if err != nil {
		t.Fatalf("ExtractToolCalls: %v", err)
	}
	if len(calls) != 1 || calls[0].Function.Name != "foo" {
		t.Errorf("calls = %v", calls)
	}
}

func TestExtractToolCallsNoChoices(t *testing.T) {
	calls, err := ExtractToolCalls([]byte(`{"choices":[]}`))
	if err != nil || calls != nil {
		t.Errorf("calls=%v err=%v, want nil,nil", calls, err)
	}
}
