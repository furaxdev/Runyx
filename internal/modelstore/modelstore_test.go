package modelstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPutGetListRemove(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ggufPath := filepath.Join(dir, "fake.gguf")
	if err := os.WriteFile(ggufPath, []byte("not a real gguf but fine for hashing"), 0o644); err != nil {
		t.Fatal(err)
	}

	mdl, err := s.ImportLocal("test-model", ggufPath)
	if err != nil {
		t.Fatalf("ImportLocal: %v", err)
	}
	if mdl.SHA256 == "" {
		t.Error("expected non-empty SHA256")
	}

	got, ok, err := s.Get("test-model")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.File != mdl.File {
		t.Errorf("File = %q, want %q", got.File, mdl.File)
	}

	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v models, err=%v", len(list), err)
	}

	if err := s.Remove("test-model", false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok, _ := s.Get("test-model"); ok {
		t.Error("model should be gone after Remove")
	}
	if _, err := os.Stat(ggufPath); err != nil {
		t.Error("file should still exist since removeFile=false")
	}
}

func TestImportLocalRejectsNonGGUF(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	txtPath := filepath.Join(dir, "notes.txt")
	os.WriteFile(txtPath, []byte("hi"), 0o644)
	if _, err := s.ImportLocal("bad", txtPath); err == nil {
		t.Error("expected error importing non-.gguf file")
	}
}

func TestScanOllamaModels(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, "manifests", "registry.ollama.ai", "library", "llama3.1")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	blobDir := filepath.Join(root, "blobs")
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	digest := "abc123"
	blobPath := filepath.Join(blobDir, "sha256-"+digest)
	if err := os.WriteFile(blobPath, []byte("fake gguf bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	mf := ollamaManifest{Layers: []ollamaManifestLayer{
		{MediaType: "application/vnd.ollama.image.model", Digest: "sha256:" + digest, Size: 123},
		{MediaType: "application/vnd.ollama.image.template", Digest: "sha256:ignored"},
	}}
	data, _ := json.Marshal(mf)
	if err := os.WriteFile(filepath.Join(manifestDir, "latest"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := ScanOllamaModels(root)
	if err != nil {
		t.Fatalf("ScanOllamaModels: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 model found, got %d", len(found))
	}
	if found[0].Name != "llama3.1:latest" {
		t.Errorf("Name = %q, want llama3.1:latest", found[0].Name)
	}
	if found[0].BlobPath != blobPath {
		t.Errorf("BlobPath = %q, want %q", found[0].BlobPath, blobPath)
	}

	s, _ := New(t.TempDir())
	mdl, err := s.ImportFromOllama(found[0])
	if err != nil {
		t.Fatalf("ImportFromOllama: %v", err)
	}
	if mdl.File != blobPath {
		t.Errorf("imported model should reference blob in place, got %q", mdl.File)
	}
}
