//go:build cgo && !windows && !guinative

package guiwindow

// openNative is the default on Linux/macOS even in a cgo-enabled build
// (e.g. `go test -race ./...`, which enables cgo for the race detector
// regardless of what the package actually needs): without the explicit
// "guinative" build tag, there is no reason to require
// libgtk-3-dev/libwebkit2gtk-4.1-dev just to compile this package, so
// Open falls through to the browser-based tiers here, same as the
// fully CGO_ENABLED=0 build (native_stub.go). Windows is unaffected --
// it gets real native support unconditionally whenever cgo is on, see
// native_cgo_windows.go.
func openNative(url, title string) error {
	return ErrNoNativeWindow
}
