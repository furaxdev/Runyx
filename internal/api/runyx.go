package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/furaxdev/runyx/internal/convert"
	"github.com/furaxdev/runyx/internal/history"
	"github.com/furaxdev/runyx/internal/vision"
	"github.com/furaxdev/runyx/internal/voice"
)

// RunyxExtras are Runyx-specific endpoints beyond the Ollama/OpenAI
// compatibility surface: local history with branches (item 14),
// universal attachments (item 17), the vision bridge (item 31), and a
// small dashboard feed (item 29). Any field left nil degrades that
// feature off rather than panicking — e.g. no Transcriber means audio
// attachments return a clear "not available" error instead of a 500.
type RunyxExtras struct {
	History     *history.Store
	Transcriber *voice.Transcriber // nil if whisper.cpp isn't installed
	Vision      *vision.Bridge     // nil if no OCR/auxiliary model is configured
}

// RegisterRunyx wires Runyx-specific routes onto mux. Call in addition
// to Server.Register for the Ollama/OpenAI surface.
func (s *Server) RegisterRunyx(mux *http.ServeMux, extras *RunyxExtras) {
	mux.HandleFunc("GET /api/runyx/conversations", extras.handleListConversations)
	mux.HandleFunc("POST /api/runyx/conversations", extras.handleNewConversation)
	mux.HandleFunc("GET /api/runyx/conversations/{id}/messages", extras.handleMessages)
	mux.HandleFunc("POST /api/runyx/conversations/{id}/messages", extras.handleAddMessage)
	mux.HandleFunc("POST /api/runyx/conversations/{id}/fork", extras.handleFork)
	mux.HandleFunc("POST /api/runyx/conversations/{id}/pin", extras.handlePin)
	mux.HandleFunc("POST /api/runyx/conversations/{id}/rename", extras.handleRename)
	mux.HandleFunc("GET /api/runyx/conversations/{id}/export", extras.handleExport)
	mux.HandleFunc("GET /api/runyx/search", extras.handleSearch)
	mux.HandleFunc("POST /api/runyx/attachments", extras.handleAttachment)
	mux.HandleFunc("GET /api/runyx/dashboard", s.handleDashboard)
}

func (e *RunyxExtras) requireHistory(w http.ResponseWriter) bool {
	if e.History == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("history is not available"))
		return false
	}
	return true
}

func (e *RunyxExtras) handleListConversations(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	convos, err := e.History.ListConversations()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, convos)
}

func (e *RunyxExtras) handleNewConversation(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	var req struct{ Title, Model, ProjectID string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	conv, err := e.History.NewConversation(req.Title, req.Model, req.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, conv)
}

func (e *RunyxExtras) handleMessages(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	id := r.PathValue("id")
	branch := r.URL.Query().Get("branch")
	msgs, err := e.History.Messages(id, branch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (e *RunyxExtras) handleAddMessage(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	id := r.PathValue("id")
	var req struct{ ParentID, Branch, Role, Content string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	msg, err := e.History.AddMessage(id, req.ParentID, req.Branch, req.Role, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

func (e *RunyxExtras) handleFork(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	id := r.PathValue("id")
	var req struct{ FromMessageID, NewBranchID, EditedContent string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := e.History.Fork(id, req.FromMessageID, req.NewBranchID, req.EditedContent); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (e *RunyxExtras) handlePin(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	id := r.PathValue("id")
	var req struct{ Pinned bool }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := e.History.Pin(id, req.Pinned); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (e *RunyxExtras) handleRename(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	id := r.PathValue("id")
	var req struct{ Title string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("title must not be empty"))
		return
	}
	if err := e.History.Rename(id, title); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (e *RunyxExtras) handleExport(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	id := r.PathValue("id")
	branch := r.URL.Query().Get("branch")
	format := r.URL.Query().Get("format")
	switch format {
	case "json":
		data, err := e.History.ExportJSON(id, branch)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		md, err := e.History.ExportMarkdown(id, branch)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Write([]byte(md))
	}
}

func (e *RunyxExtras) handleSearch(w http.ResponseWriter, r *http.Request) {
	if !e.requireHistory(w) {
		return
	}
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	results, err := e.History.Search(q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// AttachmentResponse is what the UI renders as the per-file "chip"
// showing exactly what the model received (item 17's transparency
// requirement).
type AttachmentResponse struct {
	FileName  string   `json:"file_name"`
	Kind      string   `json:"kind"`
	Text      string   `json:"text"`
	Chunks    []string `json:"chunks,omitempty"`
	Truncated bool     `json:"truncated"`
	Warnings  []string `json:"warnings,omitempty"`
}

// handleAttachment accepts a multipart file upload and returns the
// model-ready text conversion (item 17), routing images through the
// vision bridge (item 31) and audio/video through the voice pipeline
// when those are configured.
func (e *RunyxExtras) handleAttachment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing 'file' field: %w", err))
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "runyx-upload-*-"+sanitizeBase(header.Filename))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	tmp.Close()

	resp, err := e.convertOne(r.Context(), tmp.Name(), header.Filename)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (e *RunyxExtras) convertOne(ctx context.Context, tmpPath, originalName string) (AttachmentResponse, error) {
	// convert.File() dispatches by extension, but reads tmpPath's
	// (randomized) name — rename-free trick: pass tmpPath but report
	// originalName back to the caller.
	ext := strings.ToLower(filepath.Ext(originalName))

	var res convert.Result
	var err error
	switch {
	case isImageExt(ext):
		if e.Vision == nil {
			return AttachmentResponse{}, fmt.Errorf("image analysis is not configured on this server (no OCR/vision model)")
		}
		analysis, aerr := e.Vision.Describe(ctx, tmpPath)
		if aerr != nil {
			return AttachmentResponse{}, aerr
		}
		return AttachmentResponse{
			FileName: originalName, Kind: "image", Text: analysis.AsUntrustedText(),
			Warnings: nonEmpty(analysis.Warning),
		}, nil
	case isAudioExt(ext):
		if e.Transcriber == nil {
			return AttachmentResponse{}, fmt.Errorf("audio transcription is not configured on this server (whisper.cpp not installed)")
		}
		res, err = convert.AudioToText(ctx, e.Transcriber, tmpPath)
	case isVideoExt(ext):
		if e.Transcriber == nil {
			return AttachmentResponse{}, fmt.Errorf("video transcription is not configured on this server (whisper.cpp not installed)")
		}
		res, err = convert.VideoToText(ctx, e.Transcriber, tmpPath)
	default:
		res, err = convertRenamed(tmpPath, originalName)
	}
	if err != nil {
		return AttachmentResponse{}, err
	}

	const maxInline = 8000 // characters shown inline before chunking for retrieval
	resp := AttachmentResponse{FileName: originalName, Kind: res.Kind, Warnings: res.Warnings}
	if len(res.Text) > maxInline {
		resp.Chunks = convert.Chunk(res.Text, 2000, 200)
		resp.Text = res.Text[:maxInline]
		resp.Truncated = true
	} else {
		resp.Text = res.Text
	}
	return resp, nil
}

// convertRenamed calls convert.File on tmpPath but with originalName's
// extension, since convert.File dispatches purely on extension and the
// uploaded temp file's name is randomized.
func convertRenamed(tmpPath, originalName string) (convert.Result, error) {
	ext := filepath.Ext(originalName)
	renamed := tmpPath + ext
	if err := os.Rename(tmpPath, renamed); err != nil {
		return convert.Result{}, err
	}
	defer os.Rename(renamed, tmpPath)
	res, err := convert.File(renamed)
	res.FileName = originalName
	return res, err
}

func sanitizeBase(name string) string {
	name = filepath.Base(name)
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true}

func isImageExt(ext string) bool { return imageExts[ext] }
func isAudioExt(ext string) bool {
	switch ext {
	case ".mp3", ".wav", ".m4a", ".flac", ".ogg", ".opus":
		return true
	}
	return false
}
func isVideoExt(ext string) bool {
	switch ext {
	case ".mp4", ".mov", ".mkv", ".webm", ".avi":
		return true
	}
	return false
}

// DashboardStats is the discreet dashboard feed (item 29): current
// model load, resource use, and request activity.
type DashboardStats struct {
	LoadedModels []DashboardModel `json:"loaded_models"`
	TotalRAMMB   int              `json:"total_ram_mb"`
	TotalVRAMMB  int              `json:"total_vram_mb"`
	Backend      string           `json:"backend"`
}

type DashboardModel struct {
	Name    string `json:"name"`
	Port    int    `json:"port"`
	IdleSec int    `json:"idle_seconds"`
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats := DashboardStats{
		TotalRAMMB:  s.HW.TotalRAMMB,
		TotalVRAMMB: s.HW.TotalVRAMMB(),
		Backend:     s.HW.BestBackend(),
	}
	for _, inst := range s.Manager.List() {
		stats.LoadedModels = append(stats.LoadedModels, DashboardModel{
			Name: inst.ModelName, Port: inst.Port, IdleSec: int(inst.IdleFor().Seconds()),
		})
	}
	writeJSON(w, http.StatusOK, stats)
}
