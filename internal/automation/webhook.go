package automation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// WebhookRegistry holds configured inbound webhooks, each identified
// by an ID in the URL path and verified with a per-webhook shared
// secret (HMAC-SHA256 of the raw body, hex-encoded, in the
// "X-Runyx-Signature" header) so an attacker can't trigger automation
// by guessing a URL.
type WebhookRegistry struct {
	secrets map[string]string // webhook ID -> shared secret
	handler map[string]func(payload []byte) error
}

// NewWebhookRegistry returns an empty registry.
func NewWebhookRegistry() *WebhookRegistry {
	return &WebhookRegistry{secrets: map[string]string{}, handler: map[string]func([]byte) error{}}
}

// Register adds a webhook: id is the URL path segment, secret is the
// shared HMAC secret, and fn is called with the verified raw body.
func (r *WebhookRegistry) Register(id, secret string, fn func(payload []byte) error) {
	r.secrets[id] = secret
	r.handler[id] = fn
}

// Verify checks an inbound signature against the registered secret for
// id, returning an error if the webhook is unknown or the signature
// doesn't match.
func (r *WebhookRegistry) Verify(id string, body []byte, signatureHex string) error {
	secret, ok := r.secrets[id]
	if !ok {
		return fmt.Errorf("unknown webhook %q", id)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	given, err := hex.DecodeString(signatureHex)
	if err != nil || !hmac.Equal(expected, given) {
		return fmt.Errorf("invalid webhook signature")
	}
	return nil
}

// Handler returns an http.HandlerFunc serving POST /webhooks/{id},
// verifying the signature and dispatching to the registered callback.
func (r *WebhookRegistry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sig := req.Header.Get("X-Runyx-Signature")
		if err := r.Verify(id, body, sig); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		fn := r.handler[id]
		if fn == nil {
			http.Error(w, "no handler", http.StatusNotFound)
			return
		}
		if err := fn(body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
