package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/furaxdev/runyx/internal/contextmgmt"
	"github.com/furaxdev/runyx/internal/rag"
	"github.com/furaxdev/runyx/internal/websearch"
)

// RegisterKnowledge wires M3 "knowledge" endpoints onto mux: projects,
// user memory, local RAG (index + search), the context-size gauge, and
// the opt-in web search tool. Needs Server (not just RunyxExtras) since
// RAG indexing/search loads an embedding model through the same backend
// manager chat/completions use.
func (s *Server) RegisterKnowledge(mux *http.ServeMux, extras *RunyxExtras) {
	mux.HandleFunc("GET /api/runyx/projects", extras.handleListProjects)
	mux.HandleFunc("POST /api/runyx/projects", extras.handleNewProject)
	mux.HandleFunc("GET /api/runyx/memory", extras.handleListMemory)
	mux.HandleFunc("POST /api/runyx/memory", extras.handleAddMemory)
	mux.HandleFunc("DELETE /api/runyx/memory/{id}", extras.handleDeleteMemory)
	mux.HandleFunc("POST /api/runyx/rag/index", s.handleRAGIndex)
	mux.HandleFunc("GET /api/runyx/rag/search", s.handleRAGSearch)
	mux.HandleFunc("POST /api/runyx/websearch", s.handleWebSearch)
	mux.HandleFunc("POST /api/runyx/context/gauge", s.handleContextGauge)
}

func (e *RunyxExtras) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	projects, err := e.History.ListProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (e *RunyxExtras) handleNewProject(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	var req struct{ Name, SystemPrompt, FilesDir string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, err := e.History.NewProject(req.Name, req.SystemPrompt, req.FilesDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (e *RunyxExtras) handleListMemory(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	projectID := r.URL.Query().Get("project_id")
	facts, err := e.History.ListMemory(projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, facts)
}

func (e *RunyxExtras) handleAddMemory(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	var req struct{ ProjectID, Fact string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	m, err := e.History.AddMemory(req.ProjectID, req.Fact)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (e *RunyxExtras) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	if err := e.History.DeleteMemory(r.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// embedderFor loads embeddingModel through the usual backend manager
// (like any chat model) and returns a rag.Embedder pointed at it.
func (s *Server) embedderFor(r *http.Request, embeddingModel string) (*rag.Embedder, error) {
	inst, _, err := s.ensureInstance(r.Context(), embeddingModel)
	if err != nil {
		return nil, err
	}
	return &rag.Embedder{BaseURL: inst.BaseURL, Model: embeddingModel}, nil
}

func (s *Server) handleRAGIndex(w http.ResponseWriter, r *http.Request) {
	var req struct{ ProjectID, FilePath, EmbeddingModel string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	embedder, err := s.embedderFor(r, req.EmbeddingModel)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	indexer := &rag.Indexer{Store: s.History, Embedder: embedder}
	n, err := indexer.IndexFile(r.Context(), req.ProjectID, req.FilePath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"chunks_indexed": n})
}

func (s *Server) handleRAGSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	projectID := q.Get("project_id")
	query := q.Get("q")
	embeddingModel := q.Get("embedding_model")
	topK, _ := strconv.Atoi(q.Get("top_k"))

	embedder, err := s.embedderFor(r, embeddingModel)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	results, err := rag.Search(r.Context(), s.History, embedder, projectID, query, topK)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleWebSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query      string
		MaxResults int
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client := &websearch.Client{}
	results, err := client.Search(r.Context(), req.Query, req.MaxResults)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleContextGauge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages     []contextmgmt.Message
		BudgetTokens int
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	gauge := contextmgmt.ComputeGauge(req.Messages, req.BudgetTokens)
	writeJSON(w, http.StatusOK, gauge)
}
