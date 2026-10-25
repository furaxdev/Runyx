package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// AuxiliaryClient talks to a running llama-server instance that has a
// multimodal projector loaded (an already-started vision auxiliary
// model), using the OpenAI-compatible vision message format
// (content parts with an "image_url" of a data: URI), which llama-server
// supports for multimodal GGUF + mmproj pairs.
type AuxiliaryClient struct {
	BaseURL string
	Client  *http.Client
}

func (c *AuxiliaryClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

// Ask sends an image plus a question to the auxiliary vision model and
// returns its answer. This is the mechanism behind the `look_at_image`
// tool (item 31) and the initial image description both.
func (c *AuxiliaryClient) Ask(ctx context.Context, imagePath, question string) (string, error) {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image %s: %w", filepath.Base(imagePath), err)
	}
	mime := mimeFromExt(imagePath)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(imgData))

	body, _ := json.Marshal(map[string]any{
		"model": "vision-auxiliary",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": question},
					{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
				},
			},
		},
		"stream": false,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("auxiliary vision model request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("auxiliary vision model returned status %d", resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("parse auxiliary vision model response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("auxiliary vision model returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func mimeFromExt(path string) string {
	switch filepath.Ext(path) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
