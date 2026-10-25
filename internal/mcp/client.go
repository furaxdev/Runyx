package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client is an MCP client speaking JSON-RPC 2.0 over a pair of
// io.Reader/io.Writer (typically an external process's stdout/stdin).
type Client struct {
	w   io.Writer
	r   *bufio.Scanner
	cmd *exec.Cmd // nil if constructed over raw pipes (e.g. in tests)

	mu       sync.Mutex
	nextID   int64
	pending  map[int64]chan *Response
	readErr  error
	closedCh chan struct{}
}

// NewClient wraps an existing reader/writer pair (e.g. in-memory pipes)
// as an MCP client, without spawning a process.
func NewClient(w io.Writer, r io.Reader) *Client {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	c := &Client{w: w, r: sc, pending: map[int64]chan *Response{}, closedCh: make(chan struct{})}
	go c.readLoop()
	return c
}

// StartProcess launches an external MCP server (command + args) and
// returns a Client wired to its stdin/stdout.
func StartProcess(ctx context.Context, name string, args ...string) (*Client, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start MCP server %s: %w", name, err)
	}
	c := NewClient(stdin, stdout)
	c.cmd = cmd
	return c, nil
}

func (c *Client) readLoop() {
	defer close(c.closedCh)
	for c.r.Scan() {
		line := c.r.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // skip malformed/non-JSON-RPC lines
		}
		id, ok := toInt64(resp.ID)
		if !ok {
			continue
		}
		c.mu.Lock()
		ch, exists := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if exists {
			ch <- &resp
		}
	}
	c.mu.Lock()
	c.readErr = c.r.Err()
	if c.readErr == nil {
		c.readErr = io.EOF
	}
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = map[int64]chan *Response{}
	c.mu.Unlock()
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

// Call sends a JSON-RPC request and waits for its matching response.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	req := Request{JSONRPC: "2.0", ID: id, Method: method, Params: paramsJSON}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Response, 1)
	c.mu.Lock()
	if c.readErr != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("MCP connection closed: %w", c.readErr)
	}
	c.pending[id] = ch
	c.mu.Unlock()

	if _, err := c.w.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write MCP request: %w", err)
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("MCP connection closed before response arrived")
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Notify sends a JSON-RPC notification (no response expected).
func (c *Client) Notify(method string, params any) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req := Request{JSONRPC: "2.0", Method: method, Params: paramsJSON}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.w.Write(append(data, '\n'))
	return err
}

// Initialize performs the MCP handshake: "initialize" request followed
// by an "initialized" notification.
func (c *Client) Initialize(ctx context.Context) error {
	_, err := c.Call(ctx, "initialize", InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo:      ClientInfo{Name: "runyx", Version: "0.1.0"},
	})
	if err != nil {
		return fmt.Errorf("MCP initialize: %w", err)
	}
	return c.Notify("notifications/initialized", map[string]any{})
}

// ListTools calls "tools/list".
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := c.Call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result ToolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse tools/list result: %w", err)
	}
	return result.Tools, nil
}

// CallTool calls "tools/call" and returns the concatenated text content.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	raw, err := c.Call(ctx, "tools/call", ToolCallParams{Name: name, Arguments: args})
	if err != nil {
		return "", err
	}
	var result ToolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse tools/call result: %w", err)
	}
	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	if result.IsError {
		return text, fmt.Errorf("tool %q returned an error: %s", name, text)
	}
	return text, nil
}

// Close terminates the underlying process, if one was started via
// StartProcess.
func (c *Client) Close() error {
	if closer, ok := c.w.(io.Closer); ok {
		closer.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}
