//go:build cgo && !windows && guinative

package guiwindow

import (
	"fmt"

	"github.com/webview/webview_go"
)

// Gated behind the "guinative" build tag (go build -tags guinative), on
// top of cgo, rather than compiling automatically whenever cgo happens
// to be enabled: `go test -race ./...` enables cgo for the race
// detector on every platform, including CI runners that don't have
// libgtk-3-dev/libwebkit2gtk-4.1-dev installed, and this package must
// not make that otherwise-unrelated build suddenly require them. Build
// with -tags guinative explicitly (and the GTK/WebKit2GTK dev packages
// present) to get a real embedded window on Linux; without the tag,
// native_other_stub.go's stub is what actually compiles, and Open
// falls through to the browser-based tiers instead -- unchanged from
// before this package existed.
//
// openNative embeds url in a native window using webview_go's
// WebKitGTK (Linux) or Cocoa WebKit (macOS) backend. Unlike the
// Windows tier, there is no separate runtime to detect ahead of time
// here -- WebKitGTK/Cocoa WebKit are either linked at build time (and
// therefore present) or this build doesn't have cgo enabled (or the
// guinative tag) at all, in which case one of the stub variants below
// is the one actually compiled. A panic from the C/C++ side is still
// defended against on a best-effort basis, matching the Windows tier,
// even though it's less likely to be reachable on these backends.
func openNative(url, title string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: panic initializing native window: %v", ErrNoNativeWindow, r)
		}
	}()

	w := webview.New(false)
	if w == nil {
		return ErrNoNativeWindow
	}
	defer w.Destroy()

	w.SetTitle(title)
	w.SetSize(1280, 800, webview.HintNone)
	w.Navigate(url)
	w.Run()
	return nil
}
