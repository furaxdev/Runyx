// Package interpreter runs model-written Python in a restricted
// subprocess (item 22), capturing stdout/stderr and any files it
// produced (e.g. a chart saved via matplotlib).
//
// This is a *restricted* execution, not a true security sandbox: it
// isolates the working directory, caps wall-clock time, and strips the
// environment down to PATH, but it does NOT provide OS-level isolation
// (no seccomp, no cgroups/rlimits, no container/VM). A determined
// script can still make network requests or consume unbounded memory.
// Real sandboxing needs a container runtime (Docker/gVisor/Firecracker)
// or an OS sandbox API, which this project doesn't bundle — see
// docs/BLOCKED.md. Until that lands, treat this the same as the shell
// agent: never run it on code the user hasn't reviewed, and never
// expose it to a network-reachable Runyx instance without separate
// network policy.
package interpreter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Interpreter runs Python snippets inside WorkDir.
type Interpreter struct {
	PythonPath string // defaults to "python3" if empty
	WorkDir    string
	Timeout    time.Duration // defaults to 30s if zero
}

// RunResult is one execution's outcome.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
	NewFiles []string // paths (relative to WorkDir) created during the run
	Duration time.Duration
}

func (i *Interpreter) pythonPath() string {
	if i.PythonPath != "" {
		return i.PythonPath
	}
	return "python3"
}

func (i *Interpreter) timeout() time.Duration {
	if i.Timeout > 0 {
		return i.Timeout
	}
	return 30 * time.Second
}

// Run executes code and returns its result. The script is written to a
// temp file inside WorkDir (not passed via -c) so stack traces show
// sensible context and so relative file writes (e.g. a saved chart)
// land in WorkDir where NewFiles can find them.
func (i *Interpreter) Run(ctx context.Context, code string) (RunResult, error) {
	if i.WorkDir == "" {
		return RunResult{}, fmt.Errorf("interpreter: WorkDir is required")
	}
	if err := os.MkdirAll(i.WorkDir, 0o755); err != nil {
		return RunResult{}, err
	}

	before, err := listFiles(i.WorkDir)
	if err != nil {
		return RunResult{}, err
	}

	scriptPath := filepath.Join(i.WorkDir, ".runyx_script.py")
	if err := os.WriteFile(scriptPath, []byte(code), 0o644); err != nil {
		return RunResult{}, fmt.Errorf("write script: %w", err)
	}
	defer os.Remove(scriptPath)

	ctx, cancel := context.WithTimeout(ctx, i.timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, i.pythonPath(), filepath.Base(scriptPath))
	cmd.Dir = i.WorkDir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")} // strip everything else: no inherited tokens/secrets

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	result := RunResult{Stdout: stdout.String(), Stderr: stderr.String(), Duration: duration}
	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
	} else if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = ee.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}

	after, err := listFiles(i.WorkDir)
	if err == nil {
		result.NewFiles = diffFiles(before, after)
	}
	return result, nil
}

func listFiles(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, e := range entries {
		if e.Name() == ".runyx_script.py" {
			continue
		}
		set[e.Name()] = true
	}
	return set, nil
}

func diffFiles(before, after map[string]bool) []string {
	var out []string
	for name := range after {
		if !before[name] {
			out = append(out, name)
		}
	}
	return out
}
