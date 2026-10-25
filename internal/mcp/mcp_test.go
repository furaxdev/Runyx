package mcp

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"
)

// pipePair wires a Client and Server together over real io.Pipe pairs,
// so the test exercises the actual JSON-RPC line-based wire format
// end to end, not just the in-memory Go structs.
func pipePair(t *testing.T, server *Server) *Client {
	t.Helper()
	clientReadFromServer, serverWriteToClient := io.Pipe()
	serverReadFromClient, clientWriteToServer := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go server.Serve(ctx, serverWriteToClient, serverReadFromClient)

	return NewClient(clientWriteToServer, clientReadFromServer)
}

func TestClientServerInitializeAndListTools(t *testing.T) {
	srv := &Server{Tools: []ServerTool{
		{
			Name:        "echo",
			Description: "Echoes back its input",
			InputSchema: map[string]any{"type": "object"},
			Call: func(ctx context.Context, args map[string]any) (string, error) {
				return fmt.Sprintf("echo: %v", args["text"]), nil
			},
		},
	}}
	client := pipePair(t, srv)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools = %v", tools)
	}
}

func TestClientServerCallTool(t *testing.T) {
	srv := &Server{Tools: []ServerTool{
		{
			Name: "add",
			Call: func(ctx context.Context, args map[string]any) (string, error) {
				a, _ := args["a"].(float64)
				b, _ := args["b"].(float64)
				return fmt.Sprintf("%v", a+b), nil
			},
		},
	}}
	client := pipePair(t, srv)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.Initialize(ctx)

	result, err := client.CallTool(ctx, "add", map[string]any{"a": 2.0, "b": 3.0})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result != "5" {
		t.Errorf("result = %q, want 5", result)
	}
}

func TestClientServerCallUnknownTool(t *testing.T) {
	srv := &Server{}
	client := pipePair(t, srv)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.Initialize(ctx)

	_, err := client.CallTool(ctx, "nope", nil)
	if err == nil {
		t.Error("expected an error for an unknown tool")
	}
}

func TestClientServerToolReturnsError(t *testing.T) {
	srv := &Server{Tools: []ServerTool{
		{Name: "fail", Call: func(ctx context.Context, args map[string]any) (string, error) {
			return "", fmt.Errorf("something went wrong")
		}},
	}}
	client := pipePair(t, srv)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.Initialize(ctx)

	_, err := client.CallTool(ctx, "fail", nil)
	if err == nil {
		t.Fatal("expected error from a failing tool")
	}
}

func TestClientHandlesClosedConnection(t *testing.T) {
	r, w := io.Pipe()
	client := NewClient(w, r)
	w.Close()
	r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Call(ctx, "tools/list", map[string]any{}); err == nil {
		t.Error("expected error when connection is closed")
	}
}
