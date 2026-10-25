// Package api implements the Ollama-compatible (/api/*) and
// OpenAI-compatible (/v1/*) HTTP surfaces, translating requests into
// per-model llama-server calls. Implemented from each API's public
// documentation only; no Ollama source was consulted or copied.
package api

import "time"

// ChatMessage is a single turn, shared shape between the Ollama and
// OpenAI wire formats (both use "role"/"content").
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// --- Ollama wire format (https://github.com/ollama/ollama/blob/main/docs/api.md) ---

type OllamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  *bool          `json:"stream,omitempty"`
	Options map[string]any `json:"options,omitempty"`
}

type OllamaGenerateResponse struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Response  string    `json:"response"`
	Done      bool      `json:"done"`
}

type OllamaChatRequest struct {
	Model    string         `json:"model"`
	Messages []ChatMessage  `json:"messages"`
	Stream   *bool          `json:"stream,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
}

type OllamaChatResponse struct {
	Model     string      `json:"model"`
	CreatedAt time.Time   `json:"created_at"`
	Message   ChatMessage `json:"message"`
	Done      bool        `json:"done"`
}

type OllamaModelEntry struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
}

type OllamaTagsResponse struct {
	Models []OllamaModelEntry `json:"models"`
}

type OllamaShowRequest struct {
	Name string `json:"name"`
}

type OllamaShowResponse struct {
	Modelfile  string         `json:"modelfile"`
	Parameters string         `json:"parameters"`
	Details    map[string]any `json:"details"`
}

type OllamaDeleteRequest struct {
	Name string `json:"name"`
}

type OllamaPullRequest struct {
	Name   string `json:"name"`
	Stream *bool  `json:"stream,omitempty"`
}

type OllamaPullStatus struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// --- OpenAI wire format (https://platform.openai.com/docs/api-reference) ---

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelList struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}
