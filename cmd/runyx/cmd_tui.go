package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// tuiBinaryName is the sibling executable built from tui/ (see
// tui/package.json's build scripts). Looked up next to the running
// runyx binary first (the normal install layout), then on $PATH.
func tuiBinaryName() string {
	if runtime.GOOS == "windows" {
		return "runyx-tui.exe"
	}
	return "runyx-tui"
}

// findTUIBinary resolves the sibling TUI executable's path, or returns
// "" if it can't be found anywhere sensible.
func findTUIBinary() string {
	name := tuiBinaryName()
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}

// isStdoutTTY reports whether stdout looks like an interactive
// terminal rather than a pipe/file/redirect -- the TUI needs a real
// terminal to draw into, and launching it into a pipe would just
// produce garbled escape sequences in whatever's on the other end.
func isStdoutTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// ExitCodeError carries a child process's exit code so main() can
// propagate it with os.Exit without this package reaching for os.Exit
// itself (which would make cmdTUI's non-zero-exit path untestable in
// process, since os.Exit kills the test binary too).
type ExitCodeError struct{ Code int }

func (e ExitCodeError) Error() string { return fmt.Sprintf("exited with code %d", e.Code) }

// cmdTUI execs the sibling runyx-tui binary, inheriting this
// process's stdio. `runyx run <model>` is intentionally left alone
// (its own plain chat loop, with the hang safety net) -- this is
// reached only via the explicit `runyx tui` subcommand or bare `runyx`
// with no arguments on a real terminal, per docs/DECISIONS.md.
func cmdTUI(args []string) error {
	binPath := findTUIBinary()
	if binPath == "" {
		return fmt.Errorf(
			"the %s executable was not found next to runyx or on $PATH; "+
				"build it from tui/ (see tui/package.json's build scripts) and place it alongside this binary",
			tuiBinaryName(),
		)
	}
	cmd := exec.Command(binPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return ExitCodeError{Code: exitErr.ExitCode()}
		}
		return fmt.Errorf("launch %s: %w", tuiBinaryName(), err)
	}
	return nil
}
