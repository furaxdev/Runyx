package runfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.runfile.yaml")
	rf := Runfile{
		Model:        "qwen2.5:0.5b",
		Name:         "Concise assistant",
		SystemPrompt: "Answer in one sentence.",
		Parameters:   map[string]any{"temperature": 0.2},
	}
	if err := Save(path, rf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Model != rf.Model || got.SystemPrompt != rf.SystemPrompt {
		t.Errorf("got %+v, want %+v", got, rf)
	}
}

func TestLoadRejectsMissingModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("name: oops\n"), 0o644)
	if _, err := Load(path); err == nil {
		t.Error("expected error when 'model' field is missing")
	}
}

func TestImportOllamaModelfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Modelfile")
	content := `FROM llama3.1:8b
SYSTEM """You are a terse assistant."""
PARAMETER temperature 0.7
PARAMETER num_ctx 4096
ADAPTER ./my-lora.gguf
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rf, err := ImportOllamaModelfile(path)
	if err != nil {
		t.Fatalf("ImportOllamaModelfile: %v", err)
	}
	if rf.Model != "llama3.1:8b" {
		t.Errorf("Model = %q, want llama3.1:8b", rf.Model)
	}
	if rf.SystemPrompt != "You are a terse assistant." {
		t.Errorf("SystemPrompt = %q", rf.SystemPrompt)
	}
	if rf.Parameters["temperature"] != 0.7 {
		t.Errorf("temperature = %v, want 0.7", rf.Parameters["temperature"])
	}
	if rf.Parameters["num_ctx"] != 4096 {
		t.Errorf("num_ctx = %v, want 4096", rf.Parameters["num_ctx"])
	}
	if len(rf.Adapters) != 1 || rf.Adapters[0] != "./my-lora.gguf" {
		t.Errorf("Adapters = %v", rf.Adapters)
	}
}

func TestImportOllamaModelfileRequiresFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Modelfile")
	os.WriteFile(path, []byte("SYSTEM \"hi\"\n"), 0o644)
	if _, err := ImportOllamaModelfile(path); err == nil {
		t.Error("expected error when FROM directive is missing")
	}
}
