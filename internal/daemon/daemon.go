// Package daemon wires config, the model store, hardware detection and the
// backend process manager together behind an HTTP server exposing the
// Ollama- and OpenAI-compatible APIs (see internal/api).
package daemon

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/approval"
	"github.com/furaxdev/runyx/internal/automation"
	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/config"
	"github.com/furaxdev/runyx/internal/history"
	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/interpreter"
	"github.com/furaxdev/runyx/internal/modelstore"
	"github.com/furaxdev/runyx/internal/plugin"
	"github.com/furaxdev/runyx/internal/remote"
	"github.com/furaxdev/runyx/internal/vision"
	"github.com/furaxdev/runyx/internal/voice"
)

// Daemon holds everything the HTTP API needs to serve requests.
type Daemon struct {
	Config  config.Config
	HW      hwdetect.Info
	Store   *modelstore.Store
	Manager *backend.Manager
	Auth    *TokenStore
	History *history.Store

	// Transcriber and Vision are optional: nil when whisper.cpp / OCR
	// aren't available, in which case the corresponding API endpoints
	// degrade to a clear error instead of being wired up at all.
	Transcriber *voice.Transcriber
	Vision      *vision.Bridge

	// Agent and Interpreter are nil unless Config.AgentWorkDir /
	// InterpreterWorkDir are explicitly set — both are "off by
	// default, explicit opt-in" per §7.
	Agent       *agent.Agent
	Interpreter *interpreter.Interpreter

	// Approvals is the pending-approval queue every irreversible
	// action (shell commands today; Runyx Code's file/GitHub tools
	// later) blocks on until a human decides, from whichever client
	// is attached. Always set, even with Agent nil, so the HTTP API
	// never 500s on it; Agent.Approve is only wired to it when the
	// shell agent itself is enabled.
	Approvals *approval.Queue

	// Synthesizer is nil unless Piper is found on PATH.
	Synthesizer *voice.Synthesizer

	// Remote holds any configured remote-API fallback providers (item
	// 28); empty unless the user added one in config.toml. Never used
	// automatically — see internal/remote's package doc.
	Remote *remote.Router

	// Plugins and Scheduler back items 12/13. Both start empty: no
	// plugin or scheduled task runs unless explicitly added.
	Plugins   *plugin.Manager
	Scheduler *automation.Scheduler

	stopEviction context.CancelFunc
}

// New constructs a Daemon from config, ensuring directories exist and the
// llama-server backend binary is resolved. backendBinary is the path
// returned by backend.Fetcher.Ensure (resolved by the caller so New stays
// synchronous and testable without network access).
func New(cfg config.Config, backendBinary string) (*Daemon, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	store, err := modelstore.New(cfg.ModelsDir)
	if err != nil {
		return nil, err
	}
	auth, err := NewTokenStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	hw := hwdetect.Detect()
	mgr := backend.NewManager(backendBinary, time.Duration(cfg.IdleUnloadSeconds)*time.Second, cfg.VRAMBudgetMB)

	hist, err := history.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	d := &Daemon{
		Config:  cfg,
		HW:      hw,
		Store:   store,
		Manager: mgr,
		Auth:    auth,
		History: hist,
	}

	if vision.Available() {
		d.Vision = &vision.Bridge{OCR: vision.OCR{Languages: "fra+eng"}, CachePath: cfg.DataDir + "/vision_cache.json"}
	}
	if whisperBin, err := exec.LookPath("whisper-cli"); err == nil {
		d.Transcriber = &voice.Transcriber{BinaryPath: whisperBin}
	}
	if piperBin, err := exec.LookPath("piper"); err == nil {
		d.Synthesizer = &voice.Synthesizer{BinaryPath: piperBin}
	}
	d.Remote = &remote.Router{Providers: map[string]remote.Provider{}}
	d.Plugins = plugin.NewManager()
	d.Scheduler = automation.NewScheduler()

	d.Approvals = approval.NewQueue()
	if cfg.AgentWorkDir != "" {
		d.Agent = agent.New(agent.Policy{WorkDir: cfg.AgentWorkDir}, approvalAdapter(d.Approvals))
		d.Agent.AuditLogDir = cfg.DataDir
	}
	if cfg.InterpreterWorkDir != "" {
		d.Interpreter = &interpreter.Interpreter{WorkDir: cfg.InterpreterWorkDir}
	}

	return d, nil
}

// approvalAdapter bridges internal/agent's ApprovalFunc to the
// generic internal/approval queue: it submits a Request describing
// the exact command, blocks until a human resolves it (or it expires,
// or ctx is cancelled), and translates the result back to
// agent.ApprovalScope. Scope is always "once" here (never "session"
// or "always") because the approval queue has no notion of "this
// reason, for the rest of the process" — agent.Agent itself already
// remembers those on top of whatever this returns.
func approvalAdapter(q *approval.Queue) agent.ApprovalFunc {
	return func(ctx context.Context, req agent.Request) (bool, agent.ApprovalScope) {
		detail := fmt.Sprintf("command: %s\nargs: %v\nworkdir: %s\nreason: %s", req.Command, req.Args, req.WorkDir, req.Reason)
		areq := approval.NewRequest("command", req.String(), detail)
		decision, err := q.Submit(ctx, areq)
		if err != nil || !decision.Allow {
			return false, agent.ApprovalDenied
		}
		return true, agent.ApprovalOnce
	}
}

// StartEvictionLoop begins periodically evicting idle models until ctx is
// cancelled or Stop is called.
func (d *Daemon) StartEvictionLoop(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	d.stopEviction = cancel
	d.Scheduler.Start()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if evicted := d.Manager.EvictIdle(); len(evicted) > 0 {
					log.Printf("runyx: evicted idle models: %v", evicted)
				}
			}
		}
	}()
}

// Shutdown stops the eviction loop, every running model backend, and
// closes the history database.
func (d *Daemon) Shutdown() {
	if d.stopEviction != nil {
		d.stopEviction()
	}
	d.Scheduler.Stop()
	d.Plugins.StopAll()
	d.Manager.StopAll()
	if d.History != nil {
		d.History.Close()
	}
}
