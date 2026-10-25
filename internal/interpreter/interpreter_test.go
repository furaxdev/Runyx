package interpreter

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func requirePython(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available in this environment")
	}
}

func TestRunCapturesStdout(t *testing.T) {
	requirePython(t)
	interp := &Interpreter{WorkDir: t.TempDir()}
	result, err := interp.Run(context.Background(), "print('bonjour depuis python')")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, stderr=%q", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "bonjour depuis python") {
		t.Errorf("Stdout = %q", result.Stdout)
	}
}

func TestRunCapturesStderrAndNonZeroExit(t *testing.T) {
	requirePython(t)
	interp := &Interpreter{WorkDir: t.TempDir()}
	result, err := interp.Run(context.Background(), "import sys\nprint('err', file=sys.stderr)\nsys.exit(3)")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "err") {
		t.Errorf("Stderr = %q", result.Stderr)
	}
}

func TestRunDetectsNewFiles(t *testing.T) {
	requirePython(t)
	interp := &Interpreter{WorkDir: t.TempDir()}
	result, err := interp.Run(context.Background(), `
with open("output.txt", "w") as f:
    f.write("generated content")
`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.NewFiles) != 1 || result.NewFiles[0] != "output.txt" {
		t.Errorf("NewFiles = %v, want [output.txt]", result.NewFiles)
	}
}

func TestRunEnforcesTimeout(t *testing.T) {
	requirePython(t)
	interp := &Interpreter{WorkDir: t.TempDir(), Timeout: 300 * time.Millisecond}
	result, err := interp.Run(context.Background(), "import time\ntime.sleep(5)")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.TimedOut {
		t.Error("expected TimedOut=true for a script that sleeps past the timeout")
	}
}

func TestRunStripsEnvironment(t *testing.T) {
	requirePython(t)
	t.Setenv("RUNYX_TEST_SECRET", "should-not-leak")
	interp := &Interpreter{WorkDir: t.TempDir()}
	result, err := interp.Run(context.Background(), `
import os
print(os.environ.get("RUNYX_TEST_SECRET", "NOT_PRESENT"))
`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(result.Stdout, "NOT_PRESENT") {
		t.Errorf("expected the subprocess to not inherit arbitrary env vars, got stdout=%q", result.Stdout)
	}
}

func TestRunRequiresWorkDir(t *testing.T) {
	interp := &Interpreter{}
	if _, err := interp.Run(context.Background(), "print(1)"); err == nil {
		t.Error("expected error when WorkDir is empty")
	}
}
