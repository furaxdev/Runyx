package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// ServerTool is one tool the MCP server exposes. Runyx's own tool
// definitions (internal/tools.Registry) are adapted into ServerTool by
// the daemon when it wires up the MCP server endpoint, keeping this
// package free of a dependency on internal/tools.
type ServerTool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Call        func(ctx context.Context, args map[string]any) (string, error)
}

// Server implements the MCP server side of the stdio transport: reads
// JSON-RPC requests line by line from r, writes responses to w, and
// answers "initialize", "tools/list" and "tools/call" against a
// registered tool set. This is how Runyx can expose itself as an MCP
// tool server to other MCP-aware applications (item 10's "mode
// serveur").
type Server struct {
	Tools []ServerTool
}

// Serve runs the server loop until r is closed or ctx is done.
func (s *Server) Serve(ctx context.Context, w io.Writer, r io.Reader) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		resp := s.handle(ctx, req)
		if resp == nil {
			continue // notification, no response
		}
		data, err := json.Marshal(resp)
		if err != nil {
			continue
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write MCP response: %w", err)
		}
	}
	return sc.Err()
}

func (s *Server) handle(ctx context.Context, req Request) *Response {
	switch req.Method {
	case "initialize":
		return s.result(req.ID, map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "runyx", "version": "0.1.0"},
		})
	case "notifications/initialized":
		return nil
	case "tools/list":
		var tools []Tool
		for _, t := range s.Tools {
			tools = append(tools, Tool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
		}
		return s.result(req.ID, ToolsListResult{Tools: tools})
	case "tools/call":
		return s.handleToolCall(ctx, req)
	default:
		return s.errorResp(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *Server) handleToolCall(ctx context.Context, req Request) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResp(req.ID, -32602, "invalid params: "+err.Error())
	}
	for _, t := range s.Tools {
		if t.Name != params.Name {
			continue
		}
		text, err := t.Call(ctx, params.Arguments)
		if err != nil {
			return s.result(req.ID, ToolCallResult{
				Content: []ContentBlock{{Type: "text", Text: err.Error()}}, IsError: true,
			})
		}
		return s.result(req.ID, ToolCallResult{Content: []ContentBlock{{Type: "text", Text: text}}})
	}
	return s.errorResp(req.ID, -32602, "unknown tool: "+params.Name)
}

func (s *Server) result(id any, v any) *Response {
	data, _ := json.Marshal(v)
	return &Response{JSONRPC: "2.0", ID: id, Result: data}
}

func (s *Server) errorResp(id any, code int, msg string) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg}}
}
