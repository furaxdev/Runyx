//go:build cgo && !windows && !guinative

package guiwindow

import (
	"errors"
	"testing"
)

// TestOpenNativeDefaultCgoBuildReturnsErrNoNativeWindow guards the
// case this file exists for: a cgo-enabled Linux/macOS build (e.g.
// `go test -race ./...`) without the "guinative" opt-in tag must not
// try to pull in GTK/WebKit2GTK, and openNative must still report the
// same sentinel the CGO_ENABLED=0 stub does so Open's fallback chain
// behaves identically either way.
func TestOpenNativeDefaultCgoBuildReturnsErrNoNativeWindow(t *testing.T) {
	err := openNative("http://127.0.0.1:11535", "Runyx")
	if !errors.Is(err, ErrNoNativeWindow) {
		t.Errorf("openNative() = %v, want ErrNoNativeWindow", err)
	}
}
