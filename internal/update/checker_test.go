package update

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsNewerReleaseDetectsADifferentTag(t *testing.T) {
	cases := []struct {
		latestTag, current string
		want               bool
	}{
		{"v0.2.0", "0.1.0", true},
		{"v0.1.0", "0.1.0", false}, // "v" prefix normalized away
		{"0.1.0", "0.1.0", false},  // no prefix on either side
		{"v0.1.0", "dev", true},    // a dev build is never "the latest release"
		{"", "0.1.0", false},       // no tag at all (failed/empty check) -> never claim an update
		{"v0.1.0", "", true},
	}
	for _, c := range cases {
		if got := isNewerRelease(c.latestTag, c.current); got != c.want {
			t.Errorf("isNewerRelease(%q, %q) = %v, want %v", c.latestTag, c.current, got, c.want)
		}
	}
}

func TestCheckerCheckNowPopulatesResult(t *testing.T) {
	c := NewChecker("0.1.0")
	c.CheckFunc = func(_ *http.Client) (string, string, error) {
		return "v0.2.0", "https://example.invalid/runyx-linux-amd64", nil
	}

	c.CheckNow()
	res := c.Result()

	if res.CurrentVersion != "0.1.0" {
		t.Errorf("CurrentVersion = %q, want %q", res.CurrentVersion, "0.1.0")
	}
	if res.LatestVersion != "v0.2.0" {
		t.Errorf("LatestVersion = %q, want %q", res.LatestVersion, "v0.2.0")
	}
	if !res.UpdateAvailable {
		t.Error("expected UpdateAvailable to be true for a newer tag")
	}
	if res.DownloadURL == "" {
		t.Error("expected DownloadURL to be populated")
	}
	if res.CheckedAt.IsZero() {
		t.Error("expected CheckedAt to be set")
	}
	if res.Error != "" {
		t.Errorf("expected no error, got %q", res.Error)
	}
}

func TestCheckerCheckNowRecordsFailureWithoutClaimingAnUpdate(t *testing.T) {
	c := NewChecker("0.1.0")
	c.CheckFunc = func(_ *http.Client) (string, string, error) {
		return "", "", errors.New("network unreachable")
	}

	c.CheckNow()
	res := c.Result()

	if res.Error == "" {
		t.Error("expected Error to be populated when the check fails")
	}
	if res.UpdateAvailable {
		t.Error("a failed check must never report UpdateAvailable")
	}
}

func TestCheckerResultBeforeAnyCheckIsZeroValue(t *testing.T) {
	c := NewChecker("0.1.0")
	res := c.Result()
	if res.UpdateAvailable || res.LatestVersion != "" {
		t.Errorf("expected zero-value result before any check, got %+v", res)
	}
}

func TestCheckerStartChecksImmediatelyThenOnEachTick(t *testing.T) {
	var calls atomic.Int32
	c := NewChecker("0.1.0")
	c.Interval = 10 * time.Millisecond
	c.CheckFunc = func(_ *http.Client) (string, string, error) {
		calls.Add(1)
		return "v0.1.0", "", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer c.Stop()

	deadline := time.After(2 * time.Second)
	for calls.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("expected at least 3 checks within 2s, got %d", calls.Load())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
}

func TestCheckerStopEndsTheBackgroundLoop(t *testing.T) {
	var calls atomic.Int32
	c := NewChecker("0.1.0")
	c.Interval = 5 * time.Millisecond
	c.CheckFunc = func(_ *http.Client) (string, string, error) {
		calls.Add(1)
		return "v0.1.0", "", nil
	}

	c.Start(context.Background())
	time.Sleep(20 * time.Millisecond)
	c.Stop()
	after := calls.Load()
	time.Sleep(50 * time.Millisecond)
	if calls.Load() > after+1 {
		// allow at most one in-flight tick to land right after Stop
		t.Errorf("expected the check loop to stop after Stop(), calls went from %d to %d", after, calls.Load())
	}
}
