package websearch

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestSearchRealDuckDuckGo hits the real network. It's a genuine
// end-to-end test per this project's "real tests, no fake servers for
// the thing being verified" rule, but it's inherently flaky if DNS/the
// network/DDG's markup changes, so it's skippable via -short.
func TestSearchRealDuckDuckGo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in -short mode")
	}
	c := &Client{}
	var results []Result
	var err error
	// This sandbox's egress proxy to lite.duckduckgo.com is flaky
	// (intermittent resets/handshake timeouts/empty pages unrelated to
	// this package's own logic); retry a few times before failing.
	for attempt := 0; attempt < 4; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		results, err = c.Search(ctx, "golang programming", 5)
		cancel()
		if err == nil && len(results) > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one real search result")
	}
	for _, r := range results {
		if r.Title == "" || r.URL == "" {
			t.Errorf("incomplete result: %+v", r)
		}
		if !strings.HasPrefix(r.URL, "http") {
			t.Errorf("result URL not unwrapped to a direct link: %q", r.URL)
		}
	}
	block := AsContextBlock(results)
	if !strings.Contains(block, "untrusted external content") {
		t.Error("context block must self-label as untrusted per §7")
	}
}

func TestCleanDDGLinkUnwrapsRedirect(t *testing.T) {
	wrapped := "//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2F&rut=abc"
	got := cleanDDGLink(wrapped)
	if got != "https://golang.org/" {
		t.Errorf("cleanDDGLink(%q) = %q, want https://golang.org/", wrapped, got)
	}
}

func TestCleanDDGLinkPassesThroughDirect(t *testing.T) {
	direct := "https://example.com/page"
	if got := cleanDDGLink(direct); got != direct {
		t.Errorf("cleanDDGLink(direct) = %q, want unchanged", got)
	}
}

func TestCleanTextStripsTagsAndEntities(t *testing.T) {
	got := cleanText("<b>Go</b> &amp; friends")
	if got != "Go & friends" {
		t.Errorf("cleanText = %q", got)
	}
}

func TestAsContextBlockEmptyForNoResults(t *testing.T) {
	if got := AsContextBlock(nil); got != "" {
		t.Errorf("expected empty block for no results, got %q", got)
	}
}
