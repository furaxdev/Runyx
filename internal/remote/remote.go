// Package remote implements the optional, explicit opt-in remote-API
// fallback (item 28): when the local model is too weak for a request,
// route it to a remote OpenAI-compatible API using the user's own key
// instead. Never automatic — a conversation or request must explicitly
// ask for it, matching §7 ("no network request outside the ones
// requested").
package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// Provider is one configured remote endpoint.
type Provider struct {
	Name    string // display name, e.g. "OpenAI", "Anthropic-compatible gateway"
	BaseURL string // e.g. "https://api.openai.com"
	APIKey  string // the user's own key; never logged (see daemon's no-secrets-in-logs rule)
}

// Router holds the set of remote providers a user has opted into. It
// performs no routing decisions on its own — the caller (chat handler)
// decides per-request/per-conversation whether to use it, since that
// choice must always be explicit.
type Router struct {
	Providers map[string]Provider // keyed by Name
	Client    *http.Client
}

func (r *Router) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return http.DefaultClient
}

// ErrNoSuchProvider is returned by Proxy for an unconfigured provider name.
type ErrNoSuchProvider string

func (e ErrNoSuchProvider) Error() string { return "no remote provider configured named " + string(e) }

// Proxy forwards a chat completion request body to a named remote
// provider's /v1/chat/completions endpoint (OpenAI-compatible wire
// format), adding its Authorization header from the user's stored key.
// The raw response body and status code are returned for the caller to
// relay back to its own client, same as internal/api's local proxy.
func (r *Router) Proxy(ctx context.Context, providerName string, requestBody []byte) (status int, responseBody []byte, err error) {
	p, ok := r.Providers[providerName]
	if !ok {
		return 0, nil, ErrNoSuchProvider(providerName)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/v1/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := r.client().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("remote provider %q request failed: %w", providerName, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}
