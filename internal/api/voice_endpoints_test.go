package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranscribeDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	v := &VoiceExtras{}
	v.RegisterVoice(mux)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "clip.wav")
	part.Write([]byte("fake"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/runyx/voice/transcribe", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestSynthesizeDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	v := &VoiceExtras{}
	v.RegisterVoice(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/voice/synthesize", map[string]string{"Text": "bonjour"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestPairingQRCodeIssuesTokenAndReturnsPNG(t *testing.T) {
	mux := http.NewServeMux()
	var issuedFor string
	v := &VoiceExtras{
		PairHost: "192.168.1.10",
		PairPort: 11535,
		IssueToken: func(name string) (string, error) {
			issuedFor = name
			return "runyx_test_token", nil
		},
	}
	v.RegisterVoice(mux)

	body, _ := json.Marshal(map[string]string{"DeviceName": "mon-telephone"})
	req := httptest.NewRequest("POST", "/api/runyx/pairing/qrcode", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0x89, 'P', 'N', 'G'}) {
		t.Error("response body doesn't look like a PNG")
	}
	if issuedFor != "mon-telephone" {
		t.Errorf("IssueToken called with %q, want mon-telephone", issuedFor)
	}
}

func TestPairingDisabledWithoutIssueToken(t *testing.T) {
	mux := http.NewServeMux()
	v := &VoiceExtras{}
	v.RegisterVoice(mux)
	rec := doJSON(t, mux, "POST", "/api/runyx/pairing/qrcode", map[string]string{})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestPairingQRCodeMatrixReturnsModulesAndURL(t *testing.T) {
	mux := http.NewServeMux()
	v := &VoiceExtras{
		PairHost:   "192.168.1.10",
		PairPort:   11535,
		IssueToken: func(name string) (string, error) { return "runyx_test_token", nil },
	}
	v.RegisterVoice(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/pairing/qrcode-matrix", map[string]string{"DeviceName": "tui"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		URL     string
		Modules [][]bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, rec.Body.String())
	}
	if !strings.HasPrefix(body.URL, "runyx://pair?d=") {
		t.Errorf("URL = %q, unexpected prefix", body.URL)
	}
	if len(body.Modules) == 0 {
		t.Error("expected a non-empty module grid")
	}
}

func TestPairingQRCodeMatrixRefusesLoopbackListenHost(t *testing.T) {
	mux := http.NewServeMux()
	v := &VoiceExtras{
		ListenHost: "127.0.0.1",
		IssueToken: func(name string) (string, error) { return "tok", nil },
	}
	v.RegisterVoice(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/pairing/qrcode-matrix", map[string]string{})
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 when the daemon only binds loopback", rec.Code)
	}
}
