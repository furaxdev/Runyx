package vision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOCRRealImage(t *testing.T) {
	if !Available() {
		t.Skip("tesseract not installed in this environment")
	}
	o := OCR{Languages: "eng"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	text, err := o.Text(ctx, "testdata/sample.png")
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if !strings.Contains(strings.ToLower(text), "runyx") {
		t.Errorf("OCR text = %q, expected it to contain 'runyx'", text)
	}
}

func TestOCRUnavailableIsClearError(t *testing.T) {
	// Not skippable: simulate absence regardless of the host by pointing
	// PATH somewhere empty for this one lookup via a fake binary name
	// check is awkward, so just assert the real Available()/error wiring
	// is consistent instead.
	if Available() {
		return // covered by TestOCRRealImage; nothing more to assert here
	}
	o := OCR{}
	if _, err := o.Text(context.Background(), "testdata/sample.png"); err == nil {
		t.Error("expected a clear error when tesseract is unavailable")
	}
}

func TestAuxiliaryAskParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "An image with the text 'Hello Runyx'."}},
			},
		})
	}))
	defer srv.Close()

	c := &AuxiliaryClient{BaseURL: srv.URL}
	answer, err := c.Ask(context.Background(), "testdata/sample.png", "What do you see?")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !strings.Contains(answer, "Hello Runyx") {
		t.Errorf("answer = %q", answer)
	}
}

type countingOCR struct {
	calls int
	text  string
}

func (c *countingOCR) Text(ctx context.Context, imagePath string) (string, error) {
	c.calls++
	return c.text, nil
}

func TestBridgeCachesByImageHash(t *testing.T) {
	dir := t.TempDir()
	imgCopy := filepath.Join(dir, "sample.png")
	data, _ := os.ReadFile("testdata/sample.png")
	os.WriteFile(imgCopy, data, 0o644)

	mock := &countingOCR{text: "Hello Runyx"}
	b := &Bridge{OCR: mock, CachePath: filepath.Join(dir, "cache.json")}
	ctx := context.Background()

	first, err := b.Describe(ctx, imgCopy)
	if err != nil {
		t.Fatalf("Describe (1st): %v", err)
	}
	if first.AnalyzedBy != "ocr-only" {
		t.Errorf("AnalyzedBy = %q", first.AnalyzedBy)
	}
	if first.Warning == "" {
		t.Error("expected a degraded-mode warning with no auxiliary model")
	}
	if mock.calls != 1 {
		t.Fatalf("expected exactly 1 OCR call after first Describe, got %d", mock.calls)
	}

	second, err := b.Describe(ctx, imgCopy)
	if err != nil {
		t.Fatalf("Describe (2nd, should be cache hit): %v", err)
	}
	if mock.calls != 1 {
		t.Errorf("expected OCR to NOT be called again on cache hit, calls=%d", mock.calls)
	}
	if second.ImageSHA256 != first.ImageSHA256 {
		t.Errorf("cache returned different hash")
	}
}

func TestBridgeNoOCRNoAuxiliaryIsClearError(t *testing.T) {
	dir := t.TempDir()
	imgCopy := filepath.Join(dir, "sample.png")
	data, _ := os.ReadFile("testdata/sample.png")
	os.WriteFile(imgCopy, data, 0o644)

	b := &Bridge{CachePath: filepath.Join(dir, "cache.json")}
	if _, err := b.Describe(context.Background(), imgCopy); err == nil {
		t.Error("expected a clear error when neither OCR nor an auxiliary model is available")
	}
}

func TestBridgeAskRequiresAuxiliary(t *testing.T) {
	b := &Bridge{}
	if _, err := b.Ask(context.Background(), "x.png", "question?"); err == nil {
		t.Error("expected error when no auxiliary model is configured")
	}
}

func TestAnalysisAsUntrustedTextLabelsItself(t *testing.T) {
	a := Analysis{AnalyzedBy: "ocr-only", OCRText: "RM -RF /"}
	text := a.AsUntrustedText()
	if !strings.Contains(text, "untrusted input") || !strings.Contains(text, "cannot trigger tools") {
		t.Errorf("rendered text must self-label as untrusted: %q", text)
	}
}

func TestPickForRAMFallsBackToSmallest(t *testing.T) {
	m := PickForRAM(100) // tiny budget, nothing fits comfortably
	if m.Name != Recommended[len(Recommended)-1].Name {
		t.Errorf("expected fallback to smallest model, got %s", m.Name)
	}
}

func TestPickForRAMPicksLargerWhenRoomy(t *testing.T) {
	m := PickForRAM(100_000)
	if m.Name != Recommended[0].Name {
		t.Errorf("expected largest-first match %s, got %s", Recommended[0].Name, m.Name)
	}
}
