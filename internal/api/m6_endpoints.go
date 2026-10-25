package api

import (
	"encoding/json"
	"net/http"

	"github.com/furaxdev/runyx/internal/automation"
	"github.com/furaxdev/runyx/internal/bench"
	"github.com/furaxdev/runyx/internal/plugin"
)

// M6Extras wires plugins (item 12), scheduled automation (item 13),
// and bench/arena (item 7) into the HTTP API.
type M6Extras struct {
	Plugins   *plugin.Manager
	Scheduler *automation.Scheduler
	Server    *Server // for resolving/loading models to bench
}

// RegisterM6 wires the routes onto mux.
func (m *M6Extras) RegisterM6(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/runyx/plugins", m.handleListPlugins)
	mux.HandleFunc("POST /api/runyx/plugins/{name}/call", m.handleCallPlugin)
	mux.HandleFunc("POST /api/runyx/bench", m.handleBench)
	mux.HandleFunc("POST /api/runyx/arena", m.handleArena)
}

func (m *M6Extras) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	type pluginInfo struct {
		Name  string   `json:"name"`
		Tools []string `json:"tools"`
	}
	var out []pluginInfo
	if m.Plugins != nil {
		for _, running := range m.Plugins.List() {
			var toolNames []string
			for _, t := range running.Tools {
				toolNames = append(toolNames, t.Name)
			}
			out = append(out, pluginInfo{Name: running.Spec.Name, Tools: toolNames})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (m *M6Extras) handleCallPlugin(w http.ResponseWriter, r *http.Request) {
	if m.Plugins == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("plugins are not configured"))
		return
	}
	var req struct {
		Tool string
		Args map[string]any
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := m.Plugins.Call(r.Context(), r.PathValue("name"), req.Tool, req.Args)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": result})
}

func (m *M6Extras) handleBench(w http.ResponseWriter, r *http.Request) {
	var req struct{ Model, Prompt string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	inst, _, err := m.Server.ensureInstance(r.Context(), req.Model)
	if err != nil {
		writeEnsureInstanceError(w, err)
		return
	}
	prompt := req.Prompt
	if prompt == "" {
		prompt = "Explain in two sentences what a large language model is."
	}
	result, err := bench.Run(r.Context(), m.Server.Client, inst.BaseURL, req.Model, prompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (m *M6Extras) handleArena(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Models []string
		Prompt string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Models) < 2 {
		writeError(w, http.StatusBadRequest, jsonErr("arena needs at least 2 models"))
		return
	}
	var targets []bench.Target
	for _, modelName := range req.Models {
		inst, _, err := m.Server.ensureInstance(r.Context(), modelName)
		if err != nil {
			writeEnsureInstanceError(w, err)
			return
		}
		targets = append(targets, bench.Target{BaseURL: inst.BaseURL, Model: modelName})
	}
	result, err := bench.RunArena(r.Context(), m.Server.Client, req.Prompt, targets)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
