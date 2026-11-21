package update

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultCheckInterval is how often Checker re-checks GitHub for a
// newer release once it's running.
const DefaultCheckInterval = 6 * time.Hour

// CheckResult is the last known outcome of checking for a newer
// release, served as-is by the HTTP API -- never computed fresh on
// request, since that would block a page load on a live GitHub call.
type CheckResult struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version,omitempty"`
	UpdateAvailable bool      `json:"update_available"`
	DownloadURL     string    `json:"download_url,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
	Error           string    `json:"error,omitempty"`
}

// Checker periodically checks for a newer Runyx release in the
// background and caches the result. It never downloads or applies
// anything itself -- see cmd_update.go and docs/DECISIONS.md for why
// automatic apply stays refused until a real release signing key
// exists: this only ever answers "is there something newer", the same
// question `runyx update` already asks, just asked on a schedule
// instead of only when a human remembers to run the command.
type Checker struct {
	// CurrentVersion is compared against the latest release tag.
	CurrentVersion string
	// Interval between checks. Zero uses DefaultCheckInterval.
	Interval time.Duration
	// HTTPClient is used for the GitHub request. Nil uses
	// http.DefaultClient.
	HTTPClient *http.Client
	// CheckFunc is CheckLatest by default; overridable (in this
	// package's own tests, or any other package wiring up a Checker in
	// an HTTP test) so a check never has to hit the real network.
	CheckFunc func(client *http.Client) (tag, assetURL string, err error)

	mu     sync.RWMutex
	last   CheckResult
	cancel context.CancelFunc
}

// NewChecker returns a Checker that hasn't checked anything yet --
// Result() reports the zero value (UpdateAvailable: false) until Start
// or CheckNow runs at least once.
func NewChecker(currentVersion string) *Checker {
	return &Checker{CurrentVersion: currentVersion}
}

// Result returns the most recent check outcome, or the zero value if
// no check has completed yet.
func (c *Checker) Result() CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.last
}

// Start checks immediately, then every Interval, until ctx is
// cancelled or Stop is called.
func (c *Checker) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	go func() {
		c.CheckNow()
		ticker := time.NewTicker(c.interval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.CheckNow()
			}
		}
	}()
}

// Stop ends the background check loop started by Start. Safe to call
// even if Start was never called.
func (c *Checker) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// CheckNow runs one check immediately and updates Result(), without
// waiting for the next scheduled tick. Safe to call concurrently with
// a running Start loop.
func (c *Checker) CheckNow() {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	fn := c.CheckFunc
	if fn == nil {
		fn = CheckLatest
	}

	res := CheckResult{CurrentVersion: c.CurrentVersion, CheckedAt: time.Now()}
	tag, assetURL, err := fn(client)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.LatestVersion = tag
		res.DownloadURL = assetURL
		res.UpdateAvailable = isNewerRelease(tag, c.CurrentVersion)
	}

	c.mu.Lock()
	c.last = res
	c.mu.Unlock()
}

func (c *Checker) interval() time.Duration {
	if c.Interval > 0 {
		return c.Interval
	}
	return DefaultCheckInterval
}

// isNewerRelease reports whether latestTag names a different release
// than currentVersion. Tags and version.Version aren't guaranteed to
// follow strict semver in this project (a local dev build is just
// "dev"), so this deliberately doesn't parse version numbers -- it
// only normalizes a "v" prefix (GitHub release tags conventionally
// have one, version.Version never does) and compares the rest
// literally. A "dev" build is always reported as behind, since it is
// by definition not a tagged release.
func isNewerRelease(latestTag, currentVersion string) bool {
	if latestTag == "" {
		return false
	}
	if currentVersion == "dev" || currentVersion == "" {
		return true
	}
	return strings.TrimPrefix(latestTag, "v") != strings.TrimPrefix(currentVersion, "v")
}
