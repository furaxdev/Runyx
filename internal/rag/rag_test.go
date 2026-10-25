package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furaxdev/runyx/internal/history"
)

// fakeEmbeddingServer returns a deterministic embedding for each input:
// a simple bag-of-words-ish vector so that semantically similar test
// strings ("chat" vs "chien" vs "chat") score predictably, without
// needing a real embedding model.
func fakeEmbeddingServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		vec := fakeVector(req.Input)
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": vec}},
		})
	}))
}

// fakeVector hashes words into fixed buckets, giving texts that share
// words a higher cosine similarity than unrelated ones — good enough to
// prove Search() actually ranks by relevance.
func fakeVector(text string) []float32 {
	const dims = 32
	v := make([]float32, dims)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		h := 0
		for _, r := range w {
			h = h*31 + int(r)
		}
		if h < 0 {
			h = -h
		}
		v[h%dims] += 1
	}
	return v
}

func TestIndexAndSearchRanksByRelevance(t *testing.T) {
	srv := fakeEmbeddingServer(t)
	defer srv.Close()

	hist, err := history.Open(t.TempDir())
	if err != nil {
		t.Fatalf("history.Open: %v", err)
	}
	defer hist.Close()

	dir := t.TempDir()
	catsFile := filepath.Join(dir, "chats.txt")
	dogsFile := filepath.Join(dir, "chiens.txt")
	os.WriteFile(catsFile, []byte("chat dormir souris ronronner griffe moustache"), 0o644)
	os.WriteFile(dogsFile, []byte("chien aboyer jardin laisse niche croquette"), 0o644)

	embedder := &Embedder{BaseURL: srv.URL, Model: "test-embed"}
	indexer := &Indexer{Store: hist, Embedder: embedder}

	ctx := context.Background()
	if _, err := indexer.IndexFile(ctx, "proj1", catsFile); err != nil {
		t.Fatalf("IndexFile(cats): %v", err)
	}
	if _, err := indexer.IndexFile(ctx, "proj1", dogsFile); err != nil {
		t.Fatalf("IndexFile(dogs): %v", err)
	}

	results, err := Search(ctx, hist, embedder, "proj1", "chat souris ronronner", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].FileName != "chats.txt" {
		t.Errorf("top result = %q, want chats.txt (query is about cats)", results[0].FileName)
	}

	block := AsContextBlock(results)
	if !strings.Contains(block, "chats.txt") {
		t.Errorf("context block missing citation: %q", block)
	}
}

func TestSearchScopesToProject(t *testing.T) {
	srv := fakeEmbeddingServer(t)
	defer srv.Close()
	hist, _ := history.Open(t.TempDir())
	defer hist.Close()

	dir := t.TempDir()
	f := filepath.Join(dir, "note.txt")
	os.WriteFile(f, []byte("information confidentielle du projet A"), 0o644)

	embedder := &Embedder{BaseURL: srv.URL, Model: "test-embed"}
	indexer := &Indexer{Store: hist, Embedder: embedder}
	indexer.IndexFile(context.Background(), "project-a", f)

	results, err := Search(context.Background(), hist, embedder, "project-b", "information confidentielle", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results when searching a different project, got %v", results)
	}
}

func TestReindexReplacesOldChunks(t *testing.T) {
	srv := fakeEmbeddingServer(t)
	defer srv.Close()
	hist, _ := history.Open(t.TempDir())
	defer hist.Close()

	dir := t.TempDir()
	f := filepath.Join(dir, "doc.txt")
	embedder := &Embedder{BaseURL: srv.URL, Model: "test-embed"}
	indexer := &Indexer{Store: hist, Embedder: embedder}

	os.WriteFile(f, []byte("document version one"), 0o644)
	indexer.IndexFile(context.Background(), "p", f)
	os.WriteFile(f, []byte("version two, completely different"), 0o644)
	indexer.IndexFile(context.Background(), "p", f)

	chunks, err := hist.Chunks("p")
	if err != nil {
		t.Fatalf("Chunks: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected re-indexing to replace, not duplicate, got %d chunks", len(chunks))
	}
	if !strings.Contains(chunks[0].Text, "version two") {
		t.Errorf("expected the newer content, got %q", chunks[0].Text)
	}
}

func TestCosineSimilarityIdenticalVectorsScoreOne(t *testing.T) {
	v := []float32{1, 2, 3}
	score := cosineSimilarity(v, v)
	if score < 0.999 || score > 1.001 {
		t.Errorf("cosineSimilarity(v, v) = %v, want ~1.0", score)
	}
}

func TestCosineSimilarityOrthogonalScoresZero(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	score := cosineSimilarity(a, b)
	if score < -0.001 || score > 0.001 {
		t.Errorf("cosineSimilarity(orthogonal) = %v, want ~0", score)
	}
}
