// Package guiwindow shows Runyx's web UI in an app-style window instead
// of a browser tab. It tries, in order: a native embedded webview
// (WebView2 on Windows, WebKitGTK on Linux, Cocoa WebKit on macOS --
// only available in cgo-enabled builds), an installed browser's
// "app mode" (no tabs, no address bar, but still a real browser
// process), and finally the OS's default browser. Each tier is a
// strictly worse user experience than the one before it, but every
// tier works without requiring anything the user has to install or
// configure themselves.
package guiwindow

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ErrNoNativeWindow is returned by the build-tag-selected openNative
// when this build has no native webview support (no cgo) or the
// platform's native webview runtime isn't available/installed. Open
// treats it as "move to the next fallback tier", not a hard failure.
var ErrNoNativeWindow = errors.New("native window not available")

// Open shows url in an app-style window titled title, trying the
// native embedded webview first, then browser app-mode, then the OS
// default browser. It blocks until the window is closed for the
// native and app-mode tiers (so a caller can stop a daemon it started
// once the user is done); for the default-browser tier there is
// nothing to block on, since a bare "open this URL" OS command returns
// immediately and gives no way to detect when the user closes the tab.
func Open(url, title string) error {
	// Whether openNative failed because this build has no native
	// support at all (ErrNoNativeWindow) or because it tried and the
	// native runtime wasn't usable on this machine, the next tier is
	// the same: fall through to browser app mode.
	if err := openNative(url, title); err == nil {
		return nil
	}

	if err := openBrowserAppMode(url); err == nil {
		return nil
	}

	return openDefaultBrowser(url)
}

// appModeCandidates returns the browser binaries to try for app mode,
// most-preferred first, for the current OS. Edge is preferred on
// Windows since it ships with every supported Windows version and is
// the same engine WebView2 would have used; Chrome/Chromium are tried
// everywhere since either may be the only browser actually installed.
func appModeCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"msedge.exe", "chrome.exe", "google-chrome.exe"}
	case "darwin":
		return []string{
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default: // linux and other unix-likes
		return []string{"microsoft-edge", "google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}
	}
}

// openBrowserAppMode launches the first installed browser from
// appModeCandidates with --app=url, which opens a minimal window with
// no tabs and no address bar -- not a true embedded window, but close
// enough to pass for one, and needs nothing beyond a browser the user
// already has. Blocks until that browser process exits (which, for
// --app mode, is normally exactly when the user closes that window,
// as long as it's their only open browser window of that kind --
// browsers that merge into one existing process are a known gap, noted
// in docs/MANUAL_TESTS.md).
func openBrowserAppMode(url string) error {
	for _, candidate := range appModeCandidates() {
		path, err := exec.LookPath(candidate)
		if err != nil {
			// LookPath only searches $PATH; also accept an absolute path
			// that simply exists (covers the macOS .app bundle paths
			// above, which are never on $PATH).
			if !filepath.IsAbs(candidate) {
				continue
			}
			if _, statErr := os.Stat(candidate); statErr != nil {
				continue
			}
			path = candidate
		}
		cmd := exec.Command(path, "--app="+url)
		if err := cmd.Start(); err != nil {
			continue
		}
		return cmd.Wait()
	}
	return errNoBrowserFound
}

var errNoBrowserFound = errors.New("no installed browser found for app mode")

// openDefaultBrowser is the last-resort fallback: hand url to whatever
// the OS considers the default browser, via the OS's own "open this"
// command rather than guessing a browser binary ourselves. This always
// opens a normal, full browser window (tabs, address bar) -- there is
// no app-mode equivalent for "whatever the user's default is" -- and,
// unlike the two tiers above, the launching command returns
// immediately once the browser has been asked to open the URL, so
// there is nothing to block on here.
func openDefaultBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// "start" is a cmd.exe builtin, not an executable; the empty
		// string after start is the (otherwise-ambiguous) window title
		// argument, needed because start treats a quoted first argument
		// as the title rather than the target if the title is omitted.
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
