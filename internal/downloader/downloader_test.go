package downloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var staticModTime = time.Unix(0, 0)

func newReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func testContent(n int) []byte {
	b := make([]byte, n)
	r := rand.New(rand.NewSource(42))
	r.Read(b)
	return b
}

func rangeServer(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		http.ServeContent(w, r, "model.gguf", staticModTime, newReader(content))
	}))
}

func TestDownloadSingleStream(t *testing.T) {
	content := testContent(64 * 1024)
	srv := rangeServer(t, content)
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	sum := sha256.Sum256(content)
	err := Download(context.Background(), srv.Client(), srv.URL, dest, Options{
		Concurrency:    1,
		ExpectedSHA256: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(content) {
		t.Error("downloaded content mismatch")
	}
}

func TestDownloadChunkedParallel(t *testing.T) {
	content := testContent(512 * 1024)
	srv := rangeServer(t, content)
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	sum := sha256.Sum256(content)
	var lastProgress Progress
	err := Download(context.Background(), srv.Client(), srv.URL, dest, Options{
		Concurrency:    4,
		ExpectedSHA256: hex.EncodeToString(sum[:]),
		OnProgress:     func(p Progress) { lastProgress = p },
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(content) {
		t.Error("downloaded content mismatch")
	}
	if lastProgress.Total != int64(len(content)) {
		t.Errorf("progress total = %d, want %d", lastProgress.Total, len(content))
	}
}

func TestDownloadChecksumMismatch(t *testing.T) {
	content := testContent(1024)
	srv := rangeServer(t, content)
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	err := Download(context.Background(), srv.Client(), srv.URL, dest, Options{
		Concurrency:    1,
		ExpectedSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestDownloadResumesPartialFile(t *testing.T) {
	content := testContent(128 * 1024)
	srv := rangeServer(t, content)
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	part := partPath(dest)
	// Simulate a previous partial download.
	if err := os.WriteFile(part, content[:1000], 0o644); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(content)
	err := Download(context.Background(), srv.Client(), srv.URL, dest, Options{
		Concurrency:    1,
		ExpectedSHA256: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if len(got) != len(content) {
		t.Fatalf("resumed file length = %d, want %d", len(got), len(content))
	}
	if string(got) != string(content) {
		t.Error("resumed content mismatch")
	}
}
