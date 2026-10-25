package daemon

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Device is one authorized client of the daemon's HTTP API (a CLI
// install, a paired phone, a browser session), identified by a bearer
// token that can be revoked independently of the others.
type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

// TokenStore persists device tokens to disk and checks bearer tokens on
// incoming requests. Requests from loopback addresses are allowed through
// even without a token when LocalBypass is true, since the daemon's
// listen address is loopback-only by default and the OS already gates who
// can reach it; remote access (an explicit opt-in) always requires one.
type TokenStore struct {
	path        string
	LocalBypass bool

	mu      sync.Mutex
	devices []Device
}

// NewTokenStore loads (or creates) the device token file under dataDir.
func NewTokenStore(dataDir string) (*TokenStore, error) {
	ts := &TokenStore{path: filepath.Join(dataDir, "devices.json"), LocalBypass: true}
	if err := ts.load(); err != nil {
		return nil, err
	}
	return ts, nil
}

func (ts *TokenStore) load() error {
	data, err := os.ReadFile(ts.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read devices file: %w", err)
	}
	return json.Unmarshal(data, &ts.devices)
}

func (ts *TokenStore) save() error {
	data, err := json.MarshalIndent(ts.devices, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ts.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ts.path, data, 0o600)
}

// Issue creates and persists a new device token.
func (ts *TokenStore) Issue(name string) (Device, error) {
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return Device{}, err
	}
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		return Device{}, err
	}
	dev := Device{
		ID:        hex.EncodeToString(id),
		Name:      name,
		Token:     "runyx_" + hex.EncodeToString(tok),
		CreatedAt: time.Now(),
	}
	ts.mu.Lock()
	ts.devices = append(ts.devices, dev)
	err := ts.save()
	ts.mu.Unlock()
	return dev, err
}

// Revoke removes a device by ID, invalidating its token immediately.
func (ts *TokenStore) Revoke(id string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	kept := make([]Device, 0, len(ts.devices))
	for _, d := range ts.devices {
		if d.ID != id {
			kept = append(kept, d)
		}
	}
	ts.devices = kept
	return ts.save()
}

// List returns all known devices (tokens included; callers displaying
// this to a user should mask the token).
func (ts *TokenStore) List() []Device {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	out := make([]Device, len(ts.devices))
	copy(out, ts.devices)
	return out
}

// Valid reports whether token matches a known, non-revoked device.
// Comparison is constant-time (subtle.ConstantTimeCompare) so a remote
// attacker measuring response latency across many guesses can't use
// timing to recover a valid token one byte at a time -- a real,
// if slow, attack against a plain == comparison over the network.
func (ts *TokenStore) Valid(token string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	tokenBytes := []byte(token)
	for _, d := range ts.devices {
		if subtle.ConstantTimeCompare([]byte(d.Token), tokenBytes) == 1 {
			return true
		}
	}
	return false
}

// Middleware enforces bearer-token auth, with a loopback bypass when
// LocalBypass is set (the default, matching "local-only, no extra
// friction" while still requiring a token for anything reaching the API
// over the network).
func (ts *TokenStore) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ts.LocalBypass && isLoopback(r.RemoteAddr) && isGenuineLocalRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix || !ts.Valid(auth[len(prefix):]) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isGenuineLocalRequest reports whether a request whose TCP connection
// came from loopback was actually addressed to a loopback hostname by
// the client -- not just routed there at the transport layer. Without
// this check, LocalBypass is a real vulnerability: any webpage open in
// the user's browser (any origin, no user interaction beyond having
// the tab open) can issue a cross-origin fetch() straight to
// http://127.0.0.1:<port>/..., or a page served from an
// attacker-registered domain that resolves to 127.0.0.1 (DNS
// rebinding) can make the browser believe it's same-origin -- either
// way the *connection* is loopback even though the *request* was never
// meant for this daemon. The trust model behind LocalBypass only holds
// if the client itself believed it was talking to localhost, which is
// exactly what the Host header (what DNS rebinding forges) and, when
// present, the Origin header (what a direct cross-origin fetch forges)
// report. If either names something other than a loopback host, the
// bypass is refused and a real bearer token is required instead --
// this never affects a genuinely remote client (paired phone, LAN
// access), since those were never loopback at the TCP layer and so
// never reached LocalBypass in the first place.
func isGenuineLocalRequest(r *http.Request) bool {
	if !isLoopbackHostPort(r.Host) {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !isLoopbackHostname(u.Hostname()) {
			return false
		}
	}
	return true
}

func isLoopback(remoteAddr string) bool {
	host := remoteAddr
	if idx := lastColon(remoteAddr); idx >= 0 {
		host = remoteAddr[:idx]
	}
	return isLoopbackHostname(host)
}

func isLoopbackHostPort(hostport string) bool {
	host := hostport
	if idx := lastColon(hostport); idx >= 0 {
		host = hostport[:idx]
	}
	return isLoopbackHostname(host)
}

func isLoopbackHostname(host string) bool {
	host = trimBrackets(host)
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

func trimBrackets(s string) string {
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		return s[1 : len(s)-1]
	}
	return s
}
