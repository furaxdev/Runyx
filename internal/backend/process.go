package backend

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/furaxdev/runyx/internal/modelstore"
)

// Instance is one running llama-server subprocess backing a loaded model.
type Instance struct {
	ModelName string
	Port      int
	BaseURL   string

	cmd      *exec.Cmd
	exited   chan error // cmd.Wait()'s result; see the comment in Manager.Start
	lastUsed time.Time
	estMB    int
	mu       sync.Mutex
}

// Touch records that the instance was just used, for idle/LRU accounting.
func (i *Instance) Touch() {
	i.mu.Lock()
	i.lastUsed = time.Now()
	i.mu.Unlock()
}

// IdleFor returns how long the instance has gone unused.
func (i *Instance) IdleFor() time.Duration {
	i.mu.Lock()
	defer i.mu.Unlock()
	return time.Since(i.lastUsed)
}

// Manager supervises llama-server subprocesses, one per currently loaded
// model, enforcing an idle timeout and a VRAM budget via LRU eviction.
type Manager struct {
	BinaryPath    string
	IdleTimeout   time.Duration
	VRAMBudgetMB  int // 0 = unlimited
	PortRangeFrom int

	mu        sync.Mutex
	instances map[string]*Instance
	startMu   map[string]*sync.Mutex // per-model lock serializing concurrent cold starts
}

// NewManager returns a Manager that launches BinaryPath for each model.
func NewManager(binaryPath string, idleTimeout time.Duration, vramBudgetMB int) *Manager {
	return &Manager{
		BinaryPath:    binaryPath,
		IdleTimeout:   idleTimeout,
		VRAMBudgetMB:  vramBudgetMB,
		PortRangeFrom: 11540,
		instances:     make(map[string]*Instance),
		startMu:       make(map[string]*sync.Mutex),
	}
}

// startLock returns the per-model mutex used to serialize concurrent
// Start calls for the same model name, creating it on first use.
func (m *Manager) startLock(name string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.startMu[name]
	if !ok {
		l = &sync.Mutex{}
		m.startMu[name] = l
	}
	return l
}

// Get returns the running instance for a model name, if any.
func (m *Manager) Get(modelName string) (*Instance, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[modelName]
	return inst, ok
}

// List returns all currently loaded instances.
func (m *Manager) List() []*Instance {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		out = append(out, inst)
	}
	return out
}

// Start launches llama-server for mdl with the given tuning parameters,
// evicting idle/LRU instances first if the VRAM budget requires it. It
// blocks until the server's /health endpoint responds or ctx is done.
func (m *Manager) Start(ctx context.Context, mdl modelstore.Model, tuning modelstore.TuningParams) (*Instance, error) {
	// Serialize concurrent cold starts for the SAME model name: without
	// this, two concurrent first requests could both see an empty slot,
	// both launch a subprocess, and the later map write would orphan
	// the other's process (never tracked, never stopped, bypassing the
	// VRAM budget too). Different models still start concurrently since
	// each gets its own lock.
	lock := m.startLock(mdl.Name)
	lock.Lock()
	defer lock.Unlock()

	m.mu.Lock()
	if inst, ok := m.instances[mdl.Name]; ok {
		m.mu.Unlock()
		inst.Touch()
		return inst, nil
	}
	m.mu.Unlock()

	estMB := int(mdl.SizeBytes / (1024 * 1024))
	if err := m.makeRoomLocked(estMB); err != nil {
		return nil, err
	}

	port, err := freePort(m.PortRangeFrom)
	if err != nil {
		return nil, fmt.Errorf("find free port: %w", err)
	}

	args := serverArgs(mdl.File, port, tuning)

	cmd := exec.CommandContext(context.Background(), m.BinaryPath, args...) // outlives request ctx
	// Captured so a startup failure (crash, missing shared library, bad
	// flag) is diagnosable from the error alone — without this, Stdout/
	// Stderr are nil and Go silently sends them to /dev/null, so
	// "did not become healthy" looked identical whether llama-server was
	// still loading or had already died.
	output := newRingBuffer(8 * 1024)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start llama-server: %w", err)
	}

	// Exactly one goroutine ever calls cmd.Wait() (Go panics on a second
	// call), so its result is published on a buffered channel that both
	// a startup failure here and a later Stop()/StopAll() can read
	// instead of calling Wait() again. Buffered by 1 so the goroutine
	// never blocks even if nothing ever reads it.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	inst := &Instance{
		ModelName: mdl.Name,
		Port:      port,
		BaseURL:   fmt.Sprintf("http://127.0.0.1:%d", port),
		cmd:       cmd,
		exited:    exited,
		lastUsed:  time.Now(),
		estMB:     estMB,
	}

	if err := waitHealthy(ctx, inst.BaseURL, exited); err != nil {
		_ = cmd.Process.Kill()
		<-exited // reap; the Wait() goroutine above owns the only Wait() call
		out := strings.TrimSpace(output.String())
		if out == "" {
			out = "(no output)"
		}
		return nil, fmt.Errorf("llama-server did not become healthy: %w\n%s", err, diagnoseStartupFailure(out))
	}

	m.mu.Lock()
	m.instances[mdl.Name] = inst
	m.mu.Unlock()
	return inst, nil
}

// Stop terminates and removes the instance for modelName, if running.
func (m *Manager) Stop(modelName string) error {
	m.mu.Lock()
	inst, ok := m.instances[modelName]
	if ok {
		delete(m.instances, modelName)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return stopInstance(inst)
}

// StopAll terminates every running instance, e.g. on daemon shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	instances := m.instances
	m.instances = make(map[string]*Instance)
	m.mu.Unlock()
	for _, inst := range instances {
		_ = stopInstance(inst)
	}
}

func stopInstance(inst *Instance) error {
	if inst.cmd == nil || inst.cmd.Process == nil {
		return nil
	}
	err := inst.cmd.Process.Kill()
	if inst.exited != nil {
		<-inst.exited // reap; the Wait() goroutine from Start owns the only Wait() call
	}
	return err
}

// EvictIdle stops any instance that has been idle longer than
// m.IdleTimeout. Intended to be called periodically by the daemon.
func (m *Manager) EvictIdle() []string {
	if m.IdleTimeout <= 0 {
		return nil
	}
	var evicted []string
	for _, inst := range m.List() {
		if inst.IdleFor() >= m.IdleTimeout {
			if err := m.Stop(inst.ModelName); err == nil {
				evicted = append(evicted, inst.ModelName)
			}
		}
	}
	return evicted
}

// makeRoomLocked evicts least-recently-used instances until adding a model
// of size estMB would fit within the VRAM budget. No-op if VRAMBudgetMB is
// 0 (unlimited) or estMB already fits.
func (m *Manager) makeRoomLocked(estMB int) error {
	if m.VRAMBudgetMB <= 0 {
		return nil
	}
	for {
		instances := m.List()
		used := 0
		for _, inst := range instances {
			used += inst.estMB
		}
		if used+estMB <= m.VRAMBudgetMB {
			return nil
		}
		victim := lruVictim(instances)
		if victim == nil {
			return fmt.Errorf("model needs ~%d MB, exceeds VRAM budget of %d MB with nothing left to evict", estMB, m.VRAMBudgetMB)
		}
		if err := m.Stop(victim.ModelName); err != nil {
			return err
		}
	}
}

// lruVictim returns the least-recently-used instance, or nil if none.
func lruVictim(instances []*Instance) *Instance {
	var victim *Instance
	for _, inst := range instances {
		if victim == nil || inst.IdleFor() > victim.IdleFor() {
			victim = inst
		}
	}
	return victim
}

// serverArgs builds the llama-server command-line arguments for loading
// modelFile on port, tuned per tuning. Split out from Start() so the
// exact argument list (a frequent source of upstream flag-compat
// breakage, like --flash-attn's switch to requiring an explicit value)
// is unit-testable without spawning a real subprocess.
func serverArgs(modelFile string, port int, tuning modelstore.TuningParams) []string {
	args := []string{
		"--model", modelFile,
		"--port", strconv.Itoa(port),
		"--host", "127.0.0.1",
		"--ctx-size", strconv.Itoa(tuning.CtxSize),
		"--batch-size", strconv.Itoa(tuning.BatchSize),
		"--threads", strconv.Itoa(tuning.Threads),
		// Without --jinja, llama-server's /v1/chat/completions renders
		// the prompt with its own built-in guess at a chat template
		// (picked heuristically from the tokenizer, not the GGUF's
		// actual chat_template metadata). For models whose template
		// that guess doesn't match exactly -- Qwen2.5 was the one that
		// surfaced this -- the resulting prompt is missing or
		// misplaces the role markers (<|im_start|>, <|im_end|>, the
		// assistant-turn prefix), and the model generates unreadable,
		// often multilingual output despite the binary, the GGUF and
		// the machine all being fine: confirmed by hand with curl
		// against the same llama-server build (raw /completion with a
		// hand-built prompt: coherent; /v1/chat/completions with
		// --jinja, letting it read the real template from the GGUF:
		// coherent and ~8.6 tok/s; without --jinja: the garbled,
		// ~0.3 tok/s output Runyx was producing end to end). --jinja
		// makes llama-server render the model's own embedded Jinja2
		// chat_template instead, which is correct for any model that
		// ships one (Qwen, Llama 3, Gemma, Mistral, ...) with no
		// per-family template table to maintain here.
		"--jinja",
	}
	if tuning.NGPULayers != 0 {
		args = append(args, "--n-gpu-layers", strconv.Itoa(tuning.NGPULayers))
	}
	if tuning.FlashAttn {
		// Newer llama-server builds require an explicit value
		// ("on"/"off"/"auto") for --flash-attn; a bare flag (the old
		// boolean-flag convention) is now a hard parse error: "error
		// while handling argument \"--flash-attn\": expected value for
		// argument" — which killed the process before it could even
		// open its /health port.
		args = append(args, "--flash-attn", "on")
	}
	return args
}

func freePort(from int) (int, error) {
	for p := from; p < from+1000; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			l.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port found starting at %d", from)
}

// ringBuffer is an io.Writer that keeps only the last maxBytes written to
// it, so a long-running subprocess's output can be captured for
// diagnostics without the buffer growing unbounded. Safe for concurrent
// writes (exec.Cmd writes to Stdout/Stderr from its own goroutines) and
// for String() to be called after the process has been killed.
type ringBuffer struct {
	mu      sync.Mutex
	buf     []byte
	maxSize int
}

func newRingBuffer(maxSize int) *ringBuffer {
	return &ringBuffer{maxSize: maxSize}
}

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.maxSize {
		r.buf = r.buf[len(r.buf)-r.maxSize:]
	}
	return len(p), nil
}

func (r *ringBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}

// waitHealthy polls baseURL/health until it responds 200, ctx is done,
// the 30-second deadline passes, or the subprocess exits early
// (exited fires). That last case used to be invisible here: a process
// that dies instantly -- a missing shared library, a bad flag -- still
// made this function poll a connection that was never going to
// succeed for the full 30 seconds before timing out, turning an
// immediate, diagnosable failure into a generic "did not become
// healthy" 30 seconds later.
func waitHealthy(ctx context.Context, baseURL string, exited <-chan error) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-exited:
			return fmt.Errorf("process exited before becoming healthy: %w", err)
		default:
		}
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s/health", baseURL)
}

// diagnoseStartupFailure turns llama-server's raw captured stdout/
// stderr into an actionable message, recognizing the specific failure
// classes that are otherwise easy to miss in a wall of output: a
// missing dynamic library prints one line, deep inside whatever else
// the process logged before dying, and nothing about "cannot open
// shared object file" tells a non-C++ user they need to install a
// system package.
func diagnoseStartupFailure(output string) string {
	lower := strings.ToLower(output)
	if strings.Contains(lower, "libgomp.so") || (strings.Contains(lower, "libgomp") && strings.Contains(lower, "cannot open shared object file")) {
		return fmt.Sprintf("llama-server needs the GNU OpenMP runtime library (libgomp), which is not installed on this system. Install it (on Debian/Ubuntu: `sudo apt install libgomp1`; on Fedora/RHEL: `sudo dnf install libgomp`) and try again.\nllama-server output:\n%s", output)
	}
	if strings.Contains(lower, "error while loading shared libraries") || strings.Contains(lower, "cannot open shared object file") {
		return fmt.Sprintf("llama-server failed to start because a required shared library is missing on this system. See the output below for which one.\nllama-server output:\n%s", output)
	}
	return fmt.Sprintf("llama-server output:\n%s", output)
}
