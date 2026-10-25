package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/furaxdev/runyx/internal/history"
)

func newTestMux(t *testing.T) (*http.ServeMux, *history.Store) {
	t.Helper()
	hist, err := history.Open(t.TempDir())
	if err != nil {
		t.Fatalf("history.Open: %v", err)
	}
	t.Cleanup(func() { hist.Close() })

	mux := http.NewServeMux()
	s := &Server{}
	s.RegisterRunyx(mux, &RunyxExtras{History: hist})
	return mux, hist
}

func doJSON(t *testing.T, mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestConversationLifecycleOverHTTP(t *testing.T) {
	mux, _ := newTestMux(t)

	rec := doJSON(t, mux, "POST", "/api/runyx/conversations", map[string]string{"Title": "Test", "Model": "qwen2.5:0.5b"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create conversation: status %d, body %s", rec.Code, rec.Body.String())
	}
	var conv struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &conv)
	if conv.ID == "" {
		t.Fatal("expected a conversation ID")
	}

	rec = doJSON(t, mux, "POST", "/api/runyx/conversations/"+conv.ID+"/messages",
		map[string]string{"Role": "user", "Content": "Hello"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("add message: status %d, body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, mux, "GET", "/api/runyx/conversations/"+conv.ID+"/messages", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list messages: status %d", rec.Code)
	}
	var msgs []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &msgs)
	if len(msgs) != 1 || msgs[0]["Content"] != "Hello" {
		t.Errorf("messages = %v", msgs)
	}

	rec = doJSON(t, mux, "POST", "/api/runyx/conversations/"+conv.ID+"/pin", map[string]bool{"Pinned": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("pin: status %d", rec.Code)
	}

	rec = doJSON(t, mux, "GET", "/api/runyx/conversations", nil)
	var convos []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &convos)
	if len(convos) != 1 || convos[0]["Pinned"] != true {
		t.Errorf("conversations = %v", convos)
	}

	rec = doJSON(t, mux, "GET", "/api/runyx/conversations/"+conv.ID+"/export?format=markdown", nil)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("Hello")) {
		t.Errorf("export markdown: status %d, body %q", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, mux, "GET", "/api/runyx/search?q=Hello", nil)
	var results []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &results)
	if len(results) != 1 {
		t.Errorf("search results = %v", results)
	}
}

func TestRenameOverHTTP(t *testing.T) {
	mux, hist := newTestMux(t)
	conv, _ := hist.NewConversation("Original title", "m", "")

	rec := doJSON(t, mux, "POST", "/api/runyx/conversations/"+conv.ID+"/rename", map[string]string{"Title": "New title"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename: status %d, body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, mux, "GET", "/api/runyx/conversations", nil)
	var convos []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &convos)
	if len(convos) != 1 || convos[0]["Title"] != "New title" {
		t.Errorf("conversations = %v", convos)
	}
}

func TestRenameRejectsEmptyTitle(t *testing.T) {
	mux, hist := newTestMux(t)
	conv, _ := hist.NewConversation("Keep me", "m", "")

	rec := doJSON(t, mux, "POST", "/api/runyx/conversations/"+conv.ID+"/rename", map[string]string{"Title": "   "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("rename with blank title: status %d, want 400", rec.Code)
	}

	rec = doJSON(t, mux, "GET", "/api/runyx/conversations", nil)
	var convos []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &convos)
	if len(convos) != 1 || convos[0]["Title"] != "Keep me" {
		t.Errorf("expected title to be unchanged, got %v", convos)
	}
}

func TestForkOverHTTP(t *testing.T) {
	mux, hist := newTestMux(t)
	conv, _ := hist.NewConversation("t", "m", "")
	m1, _ := hist.AddMessage(conv.ID, "", "main", "user", "Capitale de la France ?")
	hist.AddMessage(conv.ID, m1.ID, "main", "assistant", "Paris")

	rec := doJSON(t, mux, "POST", "/api/runyx/conversations/"+conv.ID+"/fork", map[string]string{
		"FromMessageID": m1.ID, "NewBranchID": "edit-1", "EditedContent": "Capitale de l'Espagne ?",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("fork: status %d, body %s", rec.Code, rec.Body.String())
	}

	branches, err := hist.Branches(conv.ID)
	if err != nil || len(branches) != 2 {
		t.Fatalf("branches = %v, err=%v", branches, err)
	}
}

func TestAttachmentUploadTextFile(t *testing.T) {
	mux, _ := newTestMux(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "notes.md")
	part.Write([]byte("# Titre\n\nHello depuis le test"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/runyx/attachments", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("upload: status %d, body %s", rec.Code, rec.Body.String())
	}
	var resp AttachmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Kind != "text" || resp.FileName != "notes.md" {
		t.Errorf("resp = %+v", resp)
	}
	if resp.Text == "" {
		t.Error("expected non-empty converted text")
	}
}

func TestAttachmentUploadImageWithoutVisionConfigured(t *testing.T) {
	mux, _ := newTestMux(t)

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "photo.png")
	os.WriteFile(imgPath, []byte("not a real png but fine for this 'no vision configured' test"), 0o644)
	data, _ := os.ReadFile(imgPath)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "photo.png")
	part.Write(data)
	w.Close()

	req := httptest.NewRequest("POST", "/api/runyx/attachments", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 when no vision bridge is configured, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHistoryUnavailableReturns503(t *testing.T) {
	mux := http.NewServeMux()
	s := &Server{}
	s.RegisterRunyx(mux, &RunyxExtras{}) // no History

	rec := doJSON(t, mux, "GET", "/api/runyx/conversations", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
