//go:build cgo && windows

package guiwindow

import (
	"fmt"

	"github.com/webview/webview_go"
	"golang.org/x/sys/windows/registry"
)

// webview2ClientGUID is the Evergreen WebView2 Runtime's product GUID,
// the same one Microsoft's own detection guidance checks for: both
// per-machine and per-user Evergreen installs write their version
// string under a "pv" value at this path (WOW6432Node is used because
// the Evergreen installer is 32-bit regardless of OS/app bitness).
// Checking the registry first, before ever calling into the C++
// webview library, means a machine without the runtime falls through
// to the browser-app-mode tier cleanly instead of risking a crash deep
// in WebView2's own native initialization code, which Go's recover()
// cannot catch if it aborts the process rather than raising a C++
// exception.
const webview2ClientGUID = `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

// webview2RuntimeInstalled reports whether the WebView2 Evergreen
// Runtime appears to be installed, checking both the per-machine and
// per-user registry locations Microsoft's apps use for this exact
// check.
func webview2RuntimeInstalled() bool {
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		k, err := registry.OpenKey(root, webview2ClientGUID, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		pv, _, err := k.GetStringValue("pv")
		k.Close()
		if err == nil && pv != "" && pv != "0.0.0.0" {
			return true
		}
	}
	return false
}

// openNative embeds url in a real WebView2-backed native window: no
// tabs, no address bar, no browser chrome, and the process's own
// embedded icon (see cmd/runyx/icon_windows.rc) shows in the taskbar
// instead of a browser's. Blocks until the window is closed.
func openNative(url, title string) (err error) {
	if !webview2RuntimeInstalled() {
		return ErrNoNativeWindow
	}

	// Defense in depth beyond the registry preflight above: recover
	// catches a Go-level panic from the wrapper itself (it cannot catch
	// a C-level abort(), but the preflight check is what actually
	// prevents that case in practice).
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
