package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furaxdev/runyx/internal/remote"
)

func TestRemoteChatRequiresProvider(t *testing.T) {
	mux := http.NewServeMux()
	r := &RemoteExtras{Router: &remote.Router{Providers: map[string]remote.Provider{
		"test": {Name: "test", BaseURL: "http://unused", APIKey: "x"},
	}}}
	r.RegisterRemote(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/remote/chat", map[string]string{"model": "gpt-4"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when provider is missing", rec.Code)
	}
}

func TestRemoteChatProxiesToConfiguredProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	mux := http.NewServeMux()
	r := &RemoteExtras{Router: &remote.Router{Providers: map[string]remote.Provider{
		"test": {Name: "test", BaseURL: srv.URL, APIKey: "sk-x"},
	}}}
	r.RegisterRemote(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/remote/chat", map[string]string{"provider": "test", "model": "gpt-4"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"ok\"") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestRemoteDisabledWithNoProviders(t *testing.T) {
	mux := http.NewServeMux()
	r := &RemoteExtras{Router: &remote.Router{}}
	r.RegisterRemote(mux)
	rec := doJSON(t, mux, "POST", "/api/runyx/remote/chat", map[string]string{"provider": "x"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestListProviders(t *testing.T) {
	mux := http.NewServeMux()
	r := &RemoteExtras{Router: &remote.Router{Providers: map[string]remote.Provider{"a": {}, "b": {}}}}
	r.RegisterRemote(mux)
	rec := doJSON(t, mux, "GET", "/api/runyx/remote/providers", nil)
	var names []string
	json.Unmarshal(rec.Body.Bytes(), &names)
	if len(names) != 2 {
		t.Errorf("names = %v", names)
	}
}
