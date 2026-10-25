// Package rag implements local retrieval-augmented generation (item
// 16): indexing project files into embedded chunks, and retrieving the
// most relevant ones for a query, with citations pointing back at the
// source file and chunk. Embeddings come from a running llama-server
// instance's /v1/embeddings endpoint (any loaded embedding-capable
// model) — no external service, no telemetry.
package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"time"

	"github.com/furaxdev/runyx/internal/convert"
	"github.com/furaxdev/runyx/internal/history"
)

// Embedder calls a running embedding model.
type Embedder struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func (e *Embedder) httpClient() *http.Client {
	if e.Client != nil {
		return e.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// Embed returns the embedding vector for one piece of text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{"model": e.Model, "input": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.BaseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embedding request returned status %d", resp.StatusCode)
	}
	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("embedding response had no data")
	}
	return parsed.Data[0].Embedding, nil
}

// Indexer indexes project files into the history store's doc_chunks
// table, chunked via internal/convert.
type Indexer struct {
	Store    *history.Store
	Embedder *Embedder
	// ChunkChars/OverlapChars configure convert.Chunk; zero uses its
	// defaults (2000/0).
	ChunkChars, OverlapChars int
}

// IndexFile converts, chunks, embeds, and stores one file for a
// project. Re-indexing the same file name replaces its previous chunks.
func (idx *Indexer) IndexFile(ctx context.Context, projectID, filePath string) (int, error) {
	res, err := convert.File(filePath)
	if err != nil {
		return 0, fmt.Errorf("convert %s: %w", filepath.Base(filePath), err)
	}

	chunks := convert.Chunk(res.Text, orDefault(idx.ChunkChars, 2000), idx.OverlapChars)
	if err := idx.Store.DeleteChunksForFile(projectID, res.FileName); err != nil {
		return 0, err
	}
	for i, chunkText := range chunks {
		vec, err := idx.Embedder.Embed(ctx, chunkText)
		if err != nil {
			return i, fmt.Errorf("embed chunk %d of %s: %w", i, res.FileName, err)
		}
		if err := idx.Store.AddChunk(history.DocChunk{
			ProjectID: projectID, FileName: res.FileName, ChunkIdx: i, Text: chunkText, Embedding: vec,
		}); err != nil {
			return i, err
		}
	}
	return len(chunks), nil
}

// Citation is one retrieved passage with enough provenance to show the
// user exactly where it came from (required for item 16's "citations").
type Citation struct {
	FileName string  `json:"file_name"`
	ChunkIdx int     `json:"chunk_index"`
	Text     string  `json:"text"`
	Score    float32 `json:"score"`
}

// Search embeds query and returns the topK most similar indexed chunks
// for a project, ranked by cosine similarity descending.
func Search(ctx context.Context, store *history.Store, embedder *Embedder, projectID, query string, topK int) ([]Citation, error) {
	queryVec, err := embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	chunks, err := store.Chunks(projectID)
	if err != nil {
		return nil, err
	}

	type scored struct {
		c     history.DocChunk
		score float32
	}
	var results []scored
	for _, c := range chunks {
		results = append(results, scored{c: c, score: cosineSimilarity(queryVec, c.Embedding)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if topK <= 0 {
		topK = 5
	}
	if len(results) > topK {
		results = results[:topK]
	}
	out := make([]Citation, len(results))
	for i, r := range results {
		out[i] = Citation{FileName: r.c.FileName, ChunkIdx: r.c.ChunkIdx, Text: r.c.Text, Score: r.score}
	}
	return out, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// AsContextBlock renders citations as a text block to inject into the
// prompt, with clear provenance per passage.
func AsContextBlock(citations []Citation) string {
	if len(citations) == 0 {
		return ""
	}
	s := "[Relevant passages retrieved from indexed files]\n"
	for _, c := range citations {
		s += fmt.Sprintf("\n--- %s (passage %d) ---\n%s\n", c.FileName, c.ChunkIdx+1, c.Text)
	}
	return s
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
