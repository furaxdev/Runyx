//go:build !windows

package main

// attachParentConsole is a Windows-only concern (see console_windows.go):
// non-Windows binaries are never built with a GUI subsystem flag that
// detaches stdio, so there is nothing to reattach here.
func attachParentConsole() {}
