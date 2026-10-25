package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindTUIBinaryFindsSiblingOfRunningExecutable(t *testing.T) {
	// os.Executable() during `go test` returns the test binary's own
	// path, which is a real, stable file on disk we can use as "next
	// to the running binary" without needing to fake os.Executable.
	exePath, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable unavailable: %v", err)
	}
	dir := filepath.Dir(exePath)
	siblingPath := filepath.Join(dir, tuiBinaryName())

	if _, err := os.Stat(siblingPath); err == nil {
		t.Fatalf("test environment unexpectedly already has %s at %s", tuiBinaryName(), siblingPath)
	}
	// Create a throwaway file at the sibling path so findTUIBinary can
	// find it, then clean up.
	f, err := os.Create(siblingPath)
	if err != nil {
		t.Skipf("cannot write next to the test binary (read-only dir?): %v", err)
	}
	f.Close()
	defer os.Remove(siblingPath)

	got := findTUIBinary()
	if got != siblingPath {
		t.Errorf("findTUIBinary() = %q, want %q", got, siblingPath)
	}
}

func TestFindTUIBinaryReturnsEmptyWhenNotFoundAnywhere(t *testing.T) {
	// Isolate $PATH to a directory that definitely doesn't contain
	// runyx-tui, and the sibling-of-executable path won't have it
	// either (nothing in this test creates it there).
	emptyDir := t.TempDir()
	original := os.Getenv("PATH")
	os.Setenv("PATH", emptyDir)
	defer os.Setenv("PATH", original)

	exePath, err := os.Executable()
	if err == nil {
		sibling := filepath.Join(filepath.Dir(exePath), tuiBinaryName())
		if _, statErr := os.Stat(sibling); statErr == nil {
			t.Skip("a runyx-tui binary already exists next to the test executable")
		}
	}

	if got := findTUIBinary(); got != "" {
		t.Errorf("findTUIBinary() = %q, want empty when not found anywhere", got)
	}
}

func TestCmdTUIReportsAClearErrorWhenBinaryMissing(t *testing.T) {
	emptyDir := t.TempDir()
	original := os.Getenv("PATH")
	os.Setenv("PATH", emptyDir)
	defer os.Setenv("PATH", original)

	exePath, err := os.Executable()
	if err == nil {
		sibling := filepath.Join(filepath.Dir(exePath), tuiBinaryName())
		if _, statErr := os.Stat(sibling); statErr == nil {
			t.Skip("a runyx-tui binary already exists next to the test executable")
		}
	}

	if err := cmdTUI(nil); err == nil {
		t.Fatal("expected an error when runyx-tui cannot be found")
	}
}

func TestCmdTUIPropagatesTheStubsExitCodeAsExitCodeError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub script uses a POSIX shebang")
	}
	dir := t.TempDir()
	stubPath := filepath.Join(dir, tuiBinaryName())
	script := "#!/bin/sh\nexit 7\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	original := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+original)
	defer os.Setenv("PATH", original)

	err := cmdTUI(nil)
	exitErr, ok := err.(ExitCodeError)
	if !ok {
		t.Fatalf("cmdTUI returned %#v (%T), want an ExitCodeError", err, err)
	}
	if exitErr.Code != 7 {
		t.Errorf("exit code = %d, want 7", exitErr.Code)
	}
}

func TestCmdTUIRunsSuccessfullyWithAZeroExitStub(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub script uses a POSIX shebang")
	}
	dir := t.TempDir()
	stubPath := filepath.Join(dir, tuiBinaryName())
	marker := filepath.Join(dir, "ran")
	script := "#!/bin/sh\ntouch " + marker + "\nexit 0\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	original := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+original)
	defer os.Setenv("PATH", original)

	if err := cmdTUI(nil); err != nil {
		t.Fatalf("cmdTUI: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("expected the stub binary to have actually run")
	}
}
