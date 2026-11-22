package colibri

import "testing"

func TestServeCommandIncludesAPIKeyWhenSet(t *testing.T) {
	prog, args := ServeCommand("/usr/bin/python3", "/opt/colibri/coli", "/opt/colibri/models/glm.col", 11600, "glm-5.2-colibri", "secret-token")
	if prog != "/usr/bin/python3" {
		t.Errorf("prog = %q, want the python interpreter path", prog)
	}
	want := []string{
		"/opt/colibri/coli", "serve",
		"--model", "/opt/colibri/models/glm.col",
		"--host", "127.0.0.1",
		"--port", "11600",
		"--model-id", "glm-5.2-colibri",
		"--api-key", "secret-token",
	}
	if !equalSlices(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestServeCommandOmitsAPIKeyWhenEmpty(t *testing.T) {
	_, args := ServeCommand("/usr/bin/python3", "/opt/colibri/coli", "/opt/colibri/models/glm.col", 11600, "glm-5.2-colibri", "")
	for _, a := range args {
		if a == "--api-key" {
			t.Error("expected no --api-key flag when apiKey is empty")
		}
	}
}

func TestImportCommand(t *testing.T) {
	prog, args := ImportCommand("/usr/bin/python3", "/opt/colibri/coli", "/models/glm.gguf", "/opt/colibri/models/glm.col")
	if prog != "/usr/bin/python3" {
		t.Errorf("prog = %q", prog)
	}
	want := []string{"/opt/colibri/coli", "import", "/models/glm.gguf", "--out", "/opt/colibri/models/glm.col"}
	if !equalSlices(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestImportedModelDirIsKeyedByGGUFFileName(t *testing.T) {
	got := ImportedModelDir("/opt/colibri", "glm-5.2-q4.gguf")
	want := "/opt/colibri/models/glm-5.2-q4.gguf"
	if got != want {
		t.Errorf("ImportedModelDir() = %q, want %q", got, want)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
