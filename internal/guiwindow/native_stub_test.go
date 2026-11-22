//go:build !cgo

package guiwindow

import (
	"errors"
	"testing"
)

// TestOpenNativeStubReturnsErrNoNativeWindow guards the CGO_ENABLED=0
// build path (Runyx's default release build on every platform except
// Windows): openNative must fail with the specific sentinel Open
// checks for, not some other error, so Open reliably falls through to
// the browser-based tiers instead of surfacing a confusing failure.
func TestOpenNativeStubReturnsErrNoNativeWindow(t *testing.T) {
	err := openNative("http://127.0.0.1:11535", "Runyx")
	if !errors.Is(err, ErrNoNativeWindow) {
		t.Errorf("openNative() = %v, want ErrNoNativeWindow", err)
	}
}
