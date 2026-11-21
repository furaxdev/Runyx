package guiwindow

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestRequireDisplaySessionFailsWithNoDisplayVars guards the fix for a
// real hang: on Linux/BSD with neither DISPLAY nor WAYLAND_DISPLAY set
// (a headless box, container, or CI runner), Open must refuse to reach
// openDefaultBrowser + waitForInterrupt at all, rather than spawning
// xdg-open (which can succeed at Start() even with nothing to actually
// open a browser against) and then blocking forever with no human or
// window able to signal it to stop.
func TestRequireDisplaySessionFailsWithNoDisplayVars(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("display-session check only applies to Linux/BSD")
	}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	if err := requireDisplaySession(); err == nil {
		t.Error("expected an error with no DISPLAY or WAYLAND_DISPLAY set")
	}
}

// TestRequireDisplaySessionSucceedsWithDisplay guards the other side:
// a real X11 session (the common case, and what this sandbox's CI
// containers normally don't have -- see the no-vars test above) must
// not be refused.
func TestRequireDisplaySessionSucceedsWithDisplay(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("display-session check only applies to Linux/BSD")
	}
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")
	if err := requireDisplaySession(); err != nil {
		t.Errorf("expected no error with DISPLAY set, got %v", err)
	}
}

// TestRequireDisplaySessionAlwaysSucceedsOnWindowsAndDarwin guards the
// other branch directly: Windows and macOS both require a real GUI
// session for `start`/`open` to do anything, with no equivalent env-var
// signal, so requireDisplaySession must be a no-op there regardless of
// environment.
func TestRequireDisplaySessionAlwaysSucceedsOnWindowsAndDarwin(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("this assertion only applies on Windows/macOS")
	}
	if err := requireDisplaySession(); err != nil {
		t.Errorf("expected requireDisplaySession to be a no-op on %s, got %v", runtime.GOOS, err)
	}
}

// TestAppModeCandidatesPerOS guards against appModeCandidates silently
// returning an empty list for the current OS (which would make
// openBrowserAppMode always fail with errNoBrowserFound and fall
// through to the default-browser tier even when a real browser is
// installed): every supported OS must list at least one candidate.
func TestAppModeCandidatesPerOS(t *testing.T) {
	candidates := appModeCandidates()
	if len(candidates) == 0 {
		t.Fatalf("appModeCandidates() returned none for GOOS=%s", runtime.GOOS)
	}
	for _, c := range candidates {
		if c == "" {
			t.Error("appModeCandidates() contains an empty candidate")
		}
	}
}

// TestAppModeCandidatesDarwinAreAbsolutePaths guards the macOS branch
// specifically, since .app bundle executables are never on $PATH --
// openBrowserAppMode's fallback to checking the path directly only
// works if these are absolute paths, not bare names.
func TestAppModeCandidatesDarwinAreAbsolutePaths(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-specific candidate list")
	}
	for _, c := range appModeCandidates() {
		if !filepath.IsAbs(c) {
			t.Errorf("candidate %q is not an absolute path, needed since .app bundles aren't on $PATH", c)
		}
	}
}

// TestOpenBrowserAppModeFailsCleanlyWithNoBrowser guards the handoff
// between fallback tiers: when no candidate browser exists at all (as
// in this sandboxed test environment), openBrowserAppMode must return
// a real error rather than hang or panic, so Open can move on to the
// default-browser tier.
func TestOpenBrowserAppModeFailsCleanlyWithNoBrowser(t *testing.T) {
	// Only meaningful where none of the real candidates are installed;
	// best-effort guard against a false pass on a machine that happens
	// to have one of them (CI containers normally don't).
	err := openBrowserAppMode("http://127.0.0.1:1/does-not-matter")
	if err == nil {
		t.Skip("a candidate browser is actually installed on this machine; nothing to assert")
	}
}
