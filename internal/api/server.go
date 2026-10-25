package api

import (
	"context"
	"net/http"
	"time"

	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/history"
	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/modelstore"
)

// Server implements the Ollama- and OpenAI-compatible HTTP handlers on top
// of a model store and backend process manager.
type Server struct {
	Store   *modelstore.Store
	Manager *backend.Manager
	HW      hwdetect.Info
	Client  *http.Client
	// History is used by RegisterKnowledge (RAG indexing/search reads
	// and writes doc_chunks via the same history DB). Nil disables
	// those routes' storage — RegisterKnowledge callers should only
	// call it when History is set.
	History *history.Store
}

// NewServer returns a Server ready to have its routes registered.
func NewServer(store *modelstore.Store, mgr *backend.Manager, hw hwdetect.Info) *Server {
	return &Server{
		Store:   store,
		Manager: mgr,
		HW:      hw,
		Client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

// Register wires every /api/* and /v1/* route onto mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tags", s.handleTags)
	mux.HandleFunc("POST /api/show", s.handleShow)
	mux.HandleFunc("DELETE /api/delete", s.handleDelete)
	mux.HandleFunc("POST /api/generate", s.handleGenerate)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("POST /api/pull", s.handlePull)

	mux.HandleFunc("GET /v1/models", s.handleOpenAIModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleOpenAIProxy("/v1/chat/completions"))
	mux.HandleFunc("POST /v1/completions", s.handleOpenAIProxy("/v1/completions"))
	mux.HandleFunc("POST /v1/embeddings", s.handleOpenAIProxy("/v1/embeddings"))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

// ensureInstance resolves modelName in the store and makes sure its
// llama-server is running, tuning it (from cache or freshly computed from
// s.HW) if this is the first time it's loaded.
func (s *Server) ensureInstance(ctx context.Context, modelName string) (*backend.Instance, modelstore.Model, error) {
	mdl, ok, err := s.Store.Get(modelName)
	if err != nil {
		return nil, modelstore.Model{}, err
	}
	if !ok {
		return nil, modelstore.Model{}, errModelNotFound(modelName)
	}
	if inst, ok := s.Manager.Get(mdl.Name); ok {
		inst.Touch()
		return inst, mdl, nil
	}

	tuning := mdl.Tuning
	if tuning == (modelstore.TuningParams{}) {
		t := hwdetect.Tune(s.HW, int(mdl.SizeBytes/(1024*1024)))
		tuning = modelstore.TuningParams{
			NGPULayers: t.NGPULayers,
			Threads:    t.Threads,
			CtxSize:    t.CtxSize,
			BatchSize:  t.BatchSize,
			FlashAttn:  t.FlashAttn,
		}
		mdl.Tuning = tuning
		_ = s.Store.Put(mdl) // cache for next time; best-effort
	}

	inst, err := s.Manager.Start(ctx, mdl, tuning)
	if err != nil {
		return nil, modelstore.Model{}, err
	}
	return inst, mdl, nil
}

type notFoundError string

func (e notFoundError) Error() string { return "model not found: " + string(e) }

func errModelNotFound(name string) error { return notFoundError(name) }

// isModelNotFound reports whether err is the "no such model in the
// store" case specifically, as opposed to some other ensureInstance
// failure (e.g. the backend process failing to start) — callers use this
// to pick an HTTP status that doesn't claim "not found" for an error that
// has nothing to do with the model name.
func isModelNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

// writeEnsureInstanceError reports an ensureInstance failure with 404
// only when the model name itself wasn't found; any other failure (most
// commonly the backend process failing to start) is a server-side
// problem and must not be reported as "not found" — that misreports the
// real cause to both callers and anyone reading the error in the UI.
func writeEnsureInstanceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if isModelNotFound(err) {
		status = http.StatusNotFound
	}
	writeError(w, status, err)
}
