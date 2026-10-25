// Package mcp implements a minimal Model Context Protocol client and
// server over the stdio transport (JSON-RPC 2.0 messages, one per
// line), per the public MCP specification — no third-party SDK, so
// Runyx stays dependency-light. Implements FEATURES.md item 10: Runyx
// can both consume external MCP tool servers (Client) and expose itself
// as one (Server).
package mcp

import "encoding/json"

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

// Tool is one tool an MCP server exposes, in the shape "tools/list"
// returns it.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolsListResult is the result shape of a "tools/list" call.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams are the params of a "tools/call" request.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ContentBlock is one piece of a tool result (MCP results are a list of
// content blocks; Runyx only produces/consumes the "text" kind for now).
type ContentBlock struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// ToolCallResult is the result shape of a "tools/call" call.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// InitializeParams are the params Runyx sends as an MCP client.
type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      ClientInfo     `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ProtocolVersion is the MCP spec version Runyx implements against.
const ProtocolVersion = "2024-11-05"
