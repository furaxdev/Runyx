//go:build windows

package main

import (
	"os"
	"syscall"
)

// Runyx.exe is built with -H=windowsgui (no console subsystem) so a
// double-click from Explorer never flashes an empty console window
// before the app window appears -- but that also means a GUI-subsystem
// process starts with no console and no stdio handles at all, even
// when it was actually launched from an existing terminal to run a
// CLI subcommand like `runyx serve` or `runyx doctor`. attachConsole
// reattaches to the launching process's console when there is one
// (AttachConsole with ATTACH_PARENT_PROCESS) and reopens stdout/stderr/
// stdin against it, so CLI output is visible exactly as if this were a
// normal console-subsystem binary. When there's no parent console --
// the double-click case -- AttachConsole simply fails and this is a
// no-op, which is exactly the desired behavior there.
var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
)

const attachParentProcessID = ^uintptr(0) // ATTACH_PARENT_PROCESS (DWORD -1)

func attachParentConsole() {
	r, _, _ := procAttachConsole.Call(attachParentProcessID)
	if r == 0 {
		return
	}
	if f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
		os.Stdin = f
	}
}
