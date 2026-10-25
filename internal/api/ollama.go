package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	models, err := s.Store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := OllamaTagsResponse{}
	for _, m := range models {
		resp.Models = append(resp.Models, OllamaModelEntry{
			Name:       m.Name,
			ModifiedAt: m.ImportedAt,
			Size:       m.SizeBytes,
			Digest:     m.SHA256,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleShow(w http.ResponseWriter, r *http.Request) {
	var req OllamaShowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	mdl, ok, err := s.Store.Get(req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errModelNotFound(req.Name))
		return
	}
	writeJSON(w, http.StatusOK, OllamaShowResponse{
		Parameters: fmt.Sprintf("num_ctx %d\nnum_gpu %d", mdl.Tuning.CtxSize, mdl.Tuning.NGPULayers),
		Details: map[string]any{
			"family": "gguf",
			"source": mdl.Source,
			"size":   mdl.SizeBytes,
		},
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req OllamaDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.Manager.Stop(req.Name)
	if err := s.Store.Remove(req.Name, true); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	var req OllamaPullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	// Actual Hugging Face pulling is driven by the CLI (internal/downloader
	// + modelstore) today; this endpoint reports whatever is already
	// registered so Ollama-compatible clients get a sane response instead
	// of a 404. True server-side pulling over this API is tracked for a
	// follow-up once the UI needs it (M2).
	if _, ok, _ := s.Store.Get(req.Name); ok {
		writeJSON(w, http.StatusOK, OllamaPullStatus{Status: "success"})
		return
	}
	writeError(w, http.StatusNotFound, fmt.Errorf("%q is not installed; use `runyx pull %s` first", req.Name, req.Name))
}

// handleGenerate implements /api/generate via llama-server's native
// /completion endpoint, translating its SSE-ish NDJSON stream into
// Ollama's generate stream format.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req OllamaGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	inst, _, err := s.ensureInstance(r.Context(), req.Model)
	if err != nil {
		writeEnsureInstanceError(w, err)
		return
	}

	stream := boolOr(req.Stream, true)
	body, _ := json.Marshal(map[string]any{
		"prompt": req.Prompt,
		"stream": true, // always stream upstream; we buffer if the caller doesn't want it
	})
	upstreamResp, err := s.Client.Post(inst.BaseURL+"/completion", "application/json", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer upstreamResp.Body.Close()

	if !stream {
		var full strings.Builder
		forEachSSEField(upstreamResp.Body, "content", func(v string) { full.WriteString(v) })
		writeJSON(w, http.StatusOK, OllamaGenerateResponse{Model: req.Model, CreatedAt: time.Now(), Response: full.String(), Done: true})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	forEachSSEField(upstreamResp.Body, "content", func(v string) {
		enc.Encode(OllamaGenerateResponse{Model: req.Model, CreatedAt: time.Now(), Response: v, Done: false})
		if flusher != nil {
			flusher.Flush()
		}
	})
	enc.Encode(OllamaGenerateResponse{Model: req.Model, CreatedAt: time.Now(), Done: true})
	if flusher != nil {
		flusher.Flush()
	}
}

// handleChat implements /api/chat via llama-server's OpenAI-compatible
// /v1/chat/completions endpoint, translating its SSE delta stream into
// Ollama's chat stream format.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req OllamaChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	inst, _, err := s.ensureInstance(r.Context(), req.Model)
	if err != nil {
		writeEnsureInstanceError(w, err)
		return
	}

	stream := boolOr(req.Stream, true)
	body, _ := json.Marshal(map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	})
	upstreamResp, err := s.Client.Post(inst.BaseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer upstreamResp.Body.Close()

	extractDelta := func(data string) string {
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
			return chunk.Choices[0].Delta.Content
		}
		return ""
	}

	if !stream {
		var full strings.Builder
		forEachSSELine(upstreamResp.Body, func(data string) { full.WriteString(extractDelta(data)) })
		writeJSON(w, http.StatusOK, OllamaChatResponse{
			Model: req.Model, CreatedAt: time.Now(),
			Message: ChatMessage{Role: "assistant", Content: full.String()}, Done: true,
		})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	forEachSSELine(upstreamResp.Body, func(data string) {
		delta := extractDelta(data)
		if delta == "" {
			return
		}
		enc.Encode(OllamaChatResponse{
			Model: req.Model, CreatedAt: time.Now(),
			Message: ChatMessage{Role: "assistant", Content: delta}, Done: false,
		})
		if flusher != nil {
			flusher.Flush()
		}
	})
	enc.Encode(OllamaChatResponse{Model: req.Model, CreatedAt: time.Now(), Done: true})
	if flusher != nil {
		flusher.Flush()
	}
}

// forEachSSELine calls fn with the raw JSON payload of each "data: ..."
// line in a Server-Sent-Events stream, skipping the terminal "[DONE]".
func forEachSSELine(body io.Reader, fn func(data string)) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}
		fn(data)
	}
}

// forEachSSEField parses each SSE "data: {...}" line as JSON and calls fn
// with the given string field's value, used for llama-server's native
// /completion endpoint ({"content": "...", "stop": bool}).
func forEachSSEField(body io.Reader, field string, fn func(value string)) {
	forEachSSELine(body, func(data string) {
		var m map[string]any
		if json.Unmarshal([]byte(data), &m) != nil {
			return
		}
		if v, ok := m[field].(string); ok {
			fn(v)
		}
	})
}
