// Package plugin implements Runyx's plugin system (item 12): isolated
// subprocesses exposing tools over the same MCP stdio protocol
// internal/mcp already implements, so a plugin is just an MCP server
// Runyx launches and manages — no separate plugin protocol to design,
// test, or keep in sync with internal/mcp's client.
package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/furaxdev/runyx/internal/mcp"
)

// Spec describes one installable plugin: a command to run and how to
// launch it. A plugin registry (e.g. a JSON file under the data dir)
// is just a list of these.
type Spec struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// Running is one launched plugin: its spec, its MCP client, and the
// tools it advertised at initialize time.
type Running struct {
	Spec  Spec
	Tools []mcp.Tool

	client *mcp.Client
}

// Manager launches, tracks, and stops plugin subprocesses.
type Manager struct {
	mu      sync.Mutex
	running map[string]*Running
}

// NewManager returns an empty plugin manager.
func NewManager() *Manager {
	return &Manager{running: map[string]*Running{}}
}

// Start launches a plugin, performs the MCP handshake, and lists its
// tools. The subprocess keeps running until Stop or StopAll.
func (m *Manager) Start(ctx context.Context, spec Spec) (*Running, error) {
	m.mu.Lock()
	if existing, ok := m.running[spec.Name]; ok {
		m.mu.Unlock()
		return existing, nil
	}
	m.mu.Unlock()

	client, err := mcp.StartProcess(ctx, spec.Command, spec.Args...)
	if err != nil {
		return nil, fmt.Errorf("start plugin %q: %w", spec.Name, err)
	}
	if err := client.Initialize(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize plugin %q: %w", spec.Name, err)
	}
	tools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("list tools for plugin %q: %w", spec.Name, err)
	}

	r := &Running{Spec: spec, Tools: tools, client: client}
	m.mu.Lock()
	m.running[spec.Name] = r
	m.mu.Unlock()
	return r, nil
}

// Call invokes one tool on a running plugin.
func (m *Manager) Call(ctx context.Context, pluginName, toolName string, args map[string]any) (string, error) {
	m.mu.Lock()
	r, ok := m.running[pluginName]
	m.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("plugin %q is not running", pluginName)
	}
	return r.client.CallTool(ctx, toolName, args)
}

// List returns every currently running plugin.
func (m *Manager) List() []*Running {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Running, 0, len(m.running))
	for _, r := range m.running {
		out = append(out, r)
	}
	return out
}

// Stop terminates one plugin.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	r, ok := m.running[name]
	delete(m.running, name)
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return r.client.Close()
}

// StopAll terminates every running plugin, e.g. on daemon shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	running := m.running
	m.running = map[string]*Running{}
	m.mu.Unlock()
	for _, r := range running {
		r.client.Close()
	}
}
