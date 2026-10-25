// Package bench implements `runyx bench` (item 7): measuring
// tokens/second and time-to-first-token against a running model, and an
// "arena" mode that sends the same prompt to several models for a
// blind side-by-side comparison.
package bench

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Result is one benchmark run's measurements.
type Result struct {
	Model          string        `json:"model"`
	TimeToFirstTok time.Duration `json:"time_to_first_token"`
	TotalDuration  time.Duration `json:"total_duration"`
	TokenCount     int           `json:"token_count"` // approximated by SSE chunk count, see Run's doc comment
	TokensPerSec   float64       `json:"tokens_per_sec"`
	Text           string        `json:"text"`
}

// Run streams a chat completion against baseURL (a running llama-server
// instance) and measures latency. TokenCount counts streamed delta
// chunks, which approximates real token count closely for most
// llama-server configurations (it emits one chunk per token by
// default) but isn't guaranteed exact — good enough for a relative
// tokens/sec figure, not a certified benchmark number.
func Run(ctx context.Context, client *http.Client, baseURL, model, prompt string) (Result, error) {
	if client == nil {
		client = http.DefaultClient
	}
	body, _ := json.Marshal(map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"stream":   true,
	})

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("bench request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Result{}, fmt.Errorf("bench request returned status %d", resp.StatusCode)
	}

	var firstTokenAt time.Time
	var tokenCount int
	var text strings.Builder

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		if tokenCount == 0 {
			firstTokenAt = time.Now()
		}
		tokenCount++
		text.WriteString(delta)
	}

	total := time.Since(start)
	result := Result{Model: model, TotalDuration: total, TokenCount: tokenCount, Text: text.String()}
	if !firstTokenAt.IsZero() {
		result.TimeToFirstTok = firstTokenAt.Sub(start)
	}
	genDuration := total
	if !firstTokenAt.IsZero() {
		genDuration = total - result.TimeToFirstTok
	}
	if genDuration > 0 && tokenCount > 0 {
		result.TokensPerSec = float64(tokenCount) / genDuration.Seconds()
	}
	return result, nil
}

// ArenaResult pairs two anonymized benchmark runs for a blind
// comparison UI — labels are "A"/"B", not model names, until the user
// votes (the UI reveals which was which after voting, same pattern as
// most model arenas).
type ArenaResult struct {
	Prompt  string   `json:"prompt"`
	Entries []Result `json:"entries"`
	Labels  []string `json:"labels"` // "A", "B", ... in Entries order
}

// RunArena runs the same prompt against each (baseURL, model) pair
// concurrently and returns their anonymized results.
func RunArena(ctx context.Context, client *http.Client, prompt string, targets []Target) (ArenaResult, error) {
	results := make([]Result, len(targets))
	errs := make([]error, len(targets))
	done := make(chan int, len(targets))

	for i, t := range targets {
		i, t := i, t
		go func() {
			r, err := Run(ctx, client, t.BaseURL, t.Model, prompt)
			results[i] = r
			errs[i] = err
			done <- i
		}()
	}
	for range targets {
		<-done
	}
	for _, err := range errs {
		if err != nil {
			return ArenaResult{}, err
		}
	}

	labels := make([]string, len(targets))
	for i := range labels {
		labels[i] = string(rune('A' + i))
	}
	return ArenaResult{Prompt: prompt, Entries: results, Labels: labels}, nil
}

// Target is one model instance to include in an arena comparison.
type Target struct {
	BaseURL string
	Model   string
}
