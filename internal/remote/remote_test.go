package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyForwardsRequestAndAuth(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"remote reply"}}]}`))
	}))
	defer srv.Close()

	router := &Router{Providers: map[string]Provider{
		"test-provider": {Name: "test-provider", BaseURL: srv.URL, APIKey: "sk-test-123"},
	}}

	reqBody := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	status, body, err := router.Proxy(context.Background(), "test-provider", reqBody)
	if err != nil {
		t.Fatalf("Proxy: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("status = %d", status)
	}
	if gotAuth != "Bearer sk-test-123" {
		t.Errorf("Authorization header = %q", gotAuth)
	}
	if gotBody["model"] != "gpt-4" {
		t.Errorf("forwarded body = %v", gotBody)
	}
	if !strings.Contains(string(body), "remote reply") {
		t.Errorf("response body = %s", body)
	}
}

func TestProxyUnknownProvider(t *testing.T) {
	router := &Router{Providers: map[string]Provider{}}
	_, _, err := router.Proxy(context.Background(), "nope", nil)
	if err == nil {
		t.Error("expected error for unconfigured provider")
	}
	var notFound ErrNoSuchProvider
	if !asErrNoSuchProvider(err, &notFound) {
		t.Errorf("expected ErrNoSuchProvider, got %T: %v", err, err)
	}
}

func asErrNoSuchProvider(err error, target *ErrNoSuchProvider) bool {
	if e, ok := err.(ErrNoSuchProvider); ok {
		*target = e
		return true
	}
	return false
}
