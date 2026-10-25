package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIssueAndValidate(t *testing.T) {
	ts, err := NewTokenStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTokenStore: %v", err)
	}
	dev, err := ts.Issue("my-phone")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !ts.Valid(dev.Token) {
		t.Error("freshly issued token should be valid")
	}
	if ts.Valid("garbage") {
		t.Error("random string must not validate")
	}
}

func TestRevoke(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	dev, _ := ts.Issue("laptop")
	if err := ts.Revoke(dev.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if ts.Valid(dev.Token) {
		t.Error("revoked token must no longer validate")
	}
}

func TestMiddlewareBypassesLoopback(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true
	called := false
	h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest("GET", "/api/tags", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	// httptest.NewRequest defaults Host to "example.com" for a relative
	// target -- set it to a real loopback value, since a genuine local
	// request (the embedded web UI, the CLI, curl against 127.0.0.1)
	// always has one; see TestMiddlewareRejectsDNSRebinding for what a
	// non-loopback Host here means instead.
	req.Host = "127.0.0.1:11535"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("expected loopback request to bypass auth")
	}
}

func TestMiddlewareRejectsRemoteWithoutToken(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true
	called := false
	h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest("GET", "/api/tags", nil)
	req.RemoteAddr = "192.168.1.50:54321"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if called {
		t.Error("expected remote request without a token to be rejected")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMiddlewareAcceptsRemoteWithValidToken(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true
	dev, _ := ts.Issue("phone")
	called := false
	h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest("GET", "/api/tags", nil)
	req.RemoteAddr = "192.168.1.50:54321"
	req.Header.Set("Authorization", "Bearer "+dev.Token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("expected remote request with a valid token to pass")
	}
}

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:1234": true,
		"[::1]:1234":     true,
		"10.0.0.5:1234":  false,
	}
	for addr, want := range cases {
		if got := isLoopback(addr); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}

// TestMiddlewareRejectsDNSRebinding guards the real vulnerability this
// file's Middleware exists to close: a request whose TCP connection is
// loopback (so LocalBypass would otherwise trust it unconditionally)
// but whose Host header names something else entirely -- exactly what
// a DNS-rebinding attack looks like server-side, since the attacker's
// domain resolves to 127.0.0.1 but the browser still sends that
// domain as Host. Without the fix, this request sails through with no
// token at all.
func TestMiddlewareRejectsDNSRebinding(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true
	called := false
	h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest("GET", "/api/tags", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Host = "evil.example.com"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if called {
		t.Error("expected a loopback connection with a non-loopback Host header (DNS rebinding) to be rejected")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestMiddlewareRejectsCrossOriginBrowserFetch guards the other half of
// the same vulnerability class: a malicious webpage's JS calling
// fetch("http://127.0.0.1:<port>/...") directly. The TCP connection and
// Host header both look loopback here (the attacker can't forge
// those), but the browser itself sets Origin to the page's real
// origin, which this must catch.
func TestMiddlewareRejectsCrossOriginBrowserFetch(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true
	called := false
	h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest("GET", "/api/tags", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Host = "127.0.0.1:11535"
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if called {
		t.Error("expected a cross-origin browser fetch (Origin header from another site) to be rejected")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestMiddlewareAllowsGenuineLocalRequest is the control: the embedded
// web UI's own page, loaded from the daemon itself, must keep working
// without a token -- same Host, no cross-origin Origin header (or one
// that matches the daemon's own origin).
func TestMiddlewareAllowsGenuineLocalRequest(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true

	for _, origin := range []string{"", "http://127.0.0.1:11535"} {
		called := false
		h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
		req := httptest.NewRequest("GET", "/api/tags", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.Host = "127.0.0.1:11535"
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if !called {
			t.Errorf("Origin=%q: expected a genuine local request to bypass auth", origin)
		}
	}
}

func TestValidUsesConstantTimeCompare(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	dev, _ := ts.Issue("device")
	if !ts.Valid(dev.Token) {
		t.Error("expected the issued token to validate")
	}
	// Not a timing-attack test (that would be flaky by nature) -- just
	// confirms behavior is unchanged after switching to
	// subtle.ConstantTimeCompare: a near-miss and a garbage string both
	// still correctly fail.
	if ts.Valid(dev.Token[:len(dev.Token)-1]) {
		t.Error("a truncated token must not validate")
	}
	if ts.Valid("garbage") {
		t.Error("an unrelated string must not validate")
	}
}
