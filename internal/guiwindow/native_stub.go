//go:build !cgo

package guiwindow

// openNative is a no-op in a CGO_ENABLED=0 build (the default for
// Runyx's cross-compiled release binaries on every platform except
// Windows, where the webview build enables cgo deliberately -- see
// native_cgo_windows.go): there is no native webview implementation to
// link against without a C/C++ toolchain. Open falls through to the
// browser-based tiers, which need no cgo at all.
func openNative(url, title string) error {
	return ErrNoNativeWindow
}
