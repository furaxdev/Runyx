package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func decodeJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

func newBodyReader(b []byte) io.Reader { return bytes.NewReader(b) }

func (s *Server) handleOpenAIModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.Store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := OpenAIModelList{Object: "list"}
	for _, m := range models {
		resp.Data = append(resp.Data, OpenAIModel{
			ID:      m.Name,
			Object:  "model",
			Created: m.ImportedAt.Unix(),
			OwnedBy: "runyx",
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOpenAIProxy returns a handler that ensures the requested model is
// loaded, then transparently forwards the request body/response to the
// model's llama-server instance at upstreamPath — llama-server already
// implements the OpenAI chat/completions/embeddings surface, so no
// request/response translation is needed here, only routing and auth.
func (s *Server) handleOpenAIProxy(upstreamPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var peek struct {
			Model string `json:"model"`
		}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := decodeJSON(bodyBytes, &peek); err != nil || peek.Model == "" {
			writeError(w, http.StatusBadRequest, errMissingModel)
			return
		}

		inst, _, err := s.ensureInstance(r.Context(), peek.Model)
		if err != nil {
			writeEnsureInstanceError(w, err)
			return
		}

		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, inst.BaseURL+upstreamPath, newBodyReader(bodyBytes))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		proxyReq.Header.Set("Content-Type", "application/json")

		client := s.Client
		if client == nil {
			client = &http.Client{Timeout: 10 * time.Minute}
		}
		resp, err := client.Do(proxyReq)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		flusher, canFlush := w.(http.Flusher)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				if canFlush {
					flusher.Flush()
				}
			}
			if readErr == io.EOF {
				return
			}
			if readErr != nil {
				return
			}
		}
	}
}

var errMissingModel = errModelNotFound("<empty>")
