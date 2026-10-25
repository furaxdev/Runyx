// Package tools implements function calling and structured output
// (item 11): typed tool/function definitions, parsing tool_calls out of
// an OpenAI-shaped chat response, and a small dispatcher that runs
// registered Go handlers and feeds results back as tool messages.
//
// llama-server already accepts OpenAI's "tools"/"tool_choice" and
// "response_format": {"type": "json_schema"|"grammar", ...} fields
// as-is, and internal/api's /v1/chat/completions proxy forwards request
// bodies through unchanged — so a client can already send tool
// definitions and a JSON schema today without any Runyx-side code. This
// package is the piece that's actually new: turning Go functions into
// tool defs, and turning a model's tool_calls back into Go calls.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Def is one tool definition in the OpenAI "tools" array shape.
type Def struct {
	Type     string       `json:"type"` // always "function"
	Function FunctionSpec `json:"function"`
}

// FunctionSpec describes one callable function: name, description, and
// a JSON Schema for its parameters.
type FunctionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// Handler executes one tool call and returns the result to feed back to
// the model (as a "tool" role message). rawArgs is the tool call's
// arguments JSON, exactly as the model produced it — untrusted input
// per §7's "no tool fires without validation" rule, so a Handler must
// not perform side effects (file writes, shell commands, network calls)
// without going through its own confirmation path (see internal/agent
// for the one Handler that does: the shell tool).
type Handler func(ctx context.Context, rawArgs json.RawMessage) (string, error)

// Registry maps tool names to their definition and handler.
type Registry struct {
	defs     []Def
	handlers map[string]Handler
}

// NewRegistry returns an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{handlers: map[string]Handler{}}
}

// Register adds a tool. Re-registering the same name replaces it.
func (r *Registry) Register(spec FunctionSpec, h Handler) {
	for i, d := range r.defs {
		if d.Function.Name == spec.Name {
			r.defs[i] = Def{Type: "function", Function: spec}
			r.handlers[spec.Name] = h
			return
		}
	}
	r.defs = append(r.defs, Def{Type: "function", Function: spec})
	r.handlers[spec.Name] = h
}

// Defs returns the "tools" array to include in a chat completion request.
func (r *Registry) Defs() []Def { return r.defs }

// ToolCall mirrors the OpenAI tool_calls response shape.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON-encoded string, per the OpenAI wire format
	} `json:"function"`
}

// ExtractToolCalls parses tool_calls out of a raw
// /v1/chat/completions-shaped JSON response body.
func ExtractToolCalls(responseBody []byte) ([]ToolCall, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse chat completion response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, nil
	}
	return parsed.Choices[0].Message.ToolCalls, nil
}

// ToolResultMessage is the "tool" role message format fed back to the
// model with a call's result.
type ToolResultMessage struct {
	Role       string `json:"role"` // "tool"
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

// Dispatch runs every tool call through the registry and returns the
// resulting tool messages, in the same order as calls. A call to an
// unregistered tool, or one whose Handler errors, still returns a
// ToolResultMessage (with the error as content) rather than stopping
// the batch — the model should see what went wrong and can retry or
// explain it, rather than the whole turn silently failing.
func (r *Registry) Dispatch(ctx context.Context, calls []ToolCall) []ToolResultMessage {
	out := make([]ToolResultMessage, len(calls))
	for i, call := range calls {
		h, ok := r.handlers[call.Function.Name]
		var content string
		if !ok {
			content = fmt.Sprintf("error: unknown tool %q", call.Function.Name)
		} else {
			result, err := h(ctx, json.RawMessage(call.Function.Arguments))
			if err != nil {
				content = fmt.Sprintf("error: %v", err)
			} else {
				content = result
			}
		}
		out[i] = ToolResultMessage{Role: "tool", ToolCallID: call.ID, Content: content}
	}
	return out
}
