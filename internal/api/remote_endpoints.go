package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/furaxdev/runyx/internal/remote"
)

// RemoteExtras wires the opt-in remote-API fallback (item 28) into the
// HTTP API. Every call must name a provider explicitly — there is no
// "default remote provider" that requests silently fall back to.
type RemoteExtras struct {
	Router *remote.Router
}

// RegisterRemote wires the remote-chat route onto mux.
func (r *RemoteExtras) RegisterRemote(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/runyx/remote/providers", r.handleListProviders)
	mux.HandleFunc("POST /api/runyx/remote/chat", r.handleChat)
}

func (r *RemoteExtras) handleListProviders(w http.ResponseWriter, req *http.Request) {
	var names []string
	if r.Router != nil {
		for name := range r.Router.Providers {
			names = append(names, name)
		}
	}
	writeJSON(w, http.StatusOK, names)
}

// handleChat requires the request body to include "provider": the
// caller (UI) must have explicitly chosen to go remote for this
// request — never automatic, per §7.
func (r *RemoteExtras) handleChat(w http.ResponseWriter, req *http.Request) {
	if r.Router == nil || len(r.Router.Providers) == 0 {
		writeError(w, http.StatusServiceUnavailable, jsonErr("no remote providers are configured"))
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var peek struct{ Provider string }
	if err := json.Unmarshal(body, &peek); err != nil || peek.Provider == "" {
		writeError(w, http.StatusBadRequest, jsonErr("request must include a \"provider\" field naming which remote provider to use"))
		return
	}

	status, respBody, err := r.Router.Proxy(req.Context(), peek.Provider, body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}
