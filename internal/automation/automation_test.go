package automation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSchedulerFiresOnRealCronTick(t *testing.T) {
	s := NewScheduler()
	fired := make(chan struct{}, 1)
	// "@every 1s"-equivalent via robfig/cron's "@every" syntax, so this
	// test waits at most ~1s for a real tick rather than needing a
	// specific wall-clock minute boundary.
	if err := s.Add(Task{ID: "t1", Name: "test", CronExpr: "@every 300ms", Run: func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	}}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s.Start()
	defer s.Stop()

	select {
	case <-fired:
		// good
	case <-time.After(3 * time.Second):
		t.Fatal("task never fired within 3s")
	}

	if len(s.IDs()) != 1 {
		t.Errorf("IDs() = %v, want 1 task", s.IDs())
	}
	s.Remove("t1")
	if len(s.IDs()) != 0 {
		t.Error("expected task removed")
	}
}

func TestSchedulerRejectsBadCronExpr(t *testing.T) {
	s := NewScheduler()
	err := s.Add(Task{ID: "bad", CronExpr: "not a cron expression", Run: func() {}})
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestFolderWatcherDetectsRealFileCreation(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	var seen []string

	fw, err := WatchFolder(dir, func(path, op string) {
		mu.Lock()
		seen = append(seen, path)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("WatchFolder: %v", err)
	}
	defer fw.Close()

	newFile := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(newFile, []byte("bonjour"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		found := len(seen) > 0
		mu.Unlock()
		if found {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("expected at least one real filesystem event for the new file")
	}
	found := false
	for _, p := range seen {
		if p == newFile {
			found = true
		}
	}
	if !found {
		t.Errorf("seen = %v, want it to include %s", seen, newFile)
	}
}

func TestWebhookVerifiesRealHMACOverHTTP(t *testing.T) {
	reg := NewWebhookRegistry()
	var received []byte
	reg.Register("my-hook", "s3cr3t", func(payload []byte) error {
		received = payload
		return nil
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /webhooks/{id}", reg.Handler())
		mux.ServeHTTP(w, r)
	}))
	defer srv.Close()

	body := []byte(`{"event":"new_file","path":"/tmp/x"}`)
	mac := hmac.New(sha256.New, []byte("s3cr3t"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest("POST", srv.URL+"/webhooks/my-hook", strings.NewReader(string(body)))
	req.Header.Set("X-Runyx-Signature", sig)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if string(received) != string(body) {
		t.Errorf("received = %s, want %s", received, body)
	}
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	reg := NewWebhookRegistry()
	reg.Register("hook", "secret", func(payload []byte) error { return nil })

	srv := httptest.NewServer(reg.Handler())
	defer srv.Close()
	// Note: Handler() expects PathValue("id") which requires routing
	// through a ServeMux pattern; test the Verify function directly for
	// the signature-mismatch case to avoid needing that wiring here.
	if err := reg.Verify("hook", []byte("body"), "0000"); err == nil {
		t.Error("expected verification failure for a wrong signature")
	}
}

func TestWebhookRejectsUnknownID(t *testing.T) {
	reg := NewWebhookRegistry()
	if err := reg.Verify("nope", []byte("x"), "00"); err == nil {
		t.Error("expected error for unknown webhook id")
	}
}
