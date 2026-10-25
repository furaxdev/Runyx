package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/user"
	"strings"

	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/modelstore"
	"github.com/furaxdev/runyx/internal/registry"
)

// RegistryExtras wires the curated model catalog (models/registry.json)
// into the HTTP API: a real, versioned list the Models page renders as
// cards with a description and capability badges, instead of a bare
// "type a repo name" field.
type RegistryExtras struct {
	Server *Server
}

// RegisterRegistry wires registry + server-driven pull routes onto mux.
func (e *RegistryExtras) RegisterRegistry(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/runyx/registry", e.handleRegistry)
	mux.HandleFunc("GET /api/runyx/whoami", e.handleWhoami)
	mux.HandleFunc("POST /api/runyx/models/pull", e.handlePull)
}

// RegistryEntry is one models/registry.json model, merged with this
// machine's install state and a hardware fit assessment.
type RegistryEntry struct {
	registry.Model
	Installed bool         `json:"installed"`
	Fit       hwdetect.Fit `json:"fit"`
	PullSpec  string       `json:"pull_spec"`
}

func (e *RegistryExtras) handleRegistry(w http.ResponseWriter, r *http.Request) {
	reg, err := registry.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	installed, err := e.Server.Store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	out := make([]RegistryEntry, 0, len(reg.Models))
	for _, m := range reg.Models {
		installedName, isInstalled := resolveInstalledName(installed, m.Repo)
		pullSpec := m.PullSpec()
		if isInstalled {
			// Use the actual name this model was registered under —
			// which, for anything pulled via the CLI with no explicit
			// :file (the common case), is just the bare repo, not
			// repo:file. Sending the wrong identifier to
			// /v1/chat/completions 404s: the store has no entry under
			// the synthesized name.
			pullSpec = installedName
		}
		out = append(out, RegistryEntry{
			Model:     m,
			Installed: isInstalled,
			Fit:       hwdetect.FitFor(e.Server.HW, m.SizeBytes),
			PullSpec:  pullSpec,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// resolveInstalledName reports whether any installed model came from
// repo, and if so, the exact Name it was registered under: ImportLocal
// stores the pull spec the user actually typed ("owner/repo" or
// "owner/repo:file.gguf") as the model's Name, so any name starting
// with repo means some quantization of it is already installed — but
// the UI must address it by that stored name, not by re-deriving a
// possibly-different repo:file.
func resolveInstalledName(installed []modelstore.Model, repo string) (name string, ok bool) {
	for _, m := range installed {
		if m.Name == repo || strings.HasPrefix(m.Name, repo+":") {
			return m.Name, true
		}
	}
	return "", false
}

func (e *RegistryExtras) handleWhoami(w http.ResponseWriter, r *http.Request) {
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
		if u.Name != "" {
			name = u.Name
		}
		// A Windows/domain username like "DESKTOP\\bob" or a Unix
		// full name like "Bob Smith" — just take the first token as a
		// best-effort first name for the UI greeting.
		name = strings.FieldsFunc(name, func(r rune) bool { return r == '\\' || r == ' ' })[0]
	}
	writeJSON(w, http.StatusOK, map[string]string{"first_name": name})
}

// handlePull runs a real server-side download of a registry entry (or
// an explicit repo:file), streaming NDJSON progress lines the UI reads
// incrementally. "Pause" is the client aborting the fetch: the request
// context cancels, internal/downloader stops mid-chunk, and the partial
// `.part` + resume-state sidecar survive on disk. "Resume" is just
// calling this endpoint again — Download() picks the saved state back
// up automatically.
func (e *RegistryExtras) handlePull(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`   // registry model id, or...
		Repo string `json:"repo"` // ...an explicit repo
		File string `json:"file"` // ...+ file (Advanced / non-catalog pull)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	repo, file, name := req.Repo, req.File, ""
	if req.ID != "" {
		reg, err := registry.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		found := false
		for _, m := range reg.Models {
			if m.ID == req.ID {
				repo, file, name = m.Repo, m.File, m.PullSpec()
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, fmt.Errorf("unknown registry model id %q", req.ID))
			return
		}
	} else {
		if repo == "" || file == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("repo and file (or id) are required"))
			return
		}
		name = repo + ":" + file
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	send := func(v any) {
		enc.Encode(v)
		if flusher != nil {
			flusher.Flush()
		}
	}

	url := downloader.FileURL(repo, file)
	dest := e.Server.Store.DestPath(repo, file)

	err := downloader.Download(r.Context(), nil, url, dest, downloader.Options{
		Concurrency: 4,
		OnProgress: func(p downloader.Progress) {
			send(map[string]any{"downloaded": p.Downloaded, "total": p.Total, "done": false})
		},
	})
	if err != nil {
		send(map[string]any{"error": err.Error(), "done": true})
		return
	}

	mdl, err := e.Server.Store.ImportLocal(name, dest)
	if err != nil {
		send(map[string]any{"error": err.Error(), "done": true})
		return
	}
	send(map[string]any{"done": true, "name": mdl.Name})
}
