package colibri

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// ImportCommand returns the program and arguments to convert a raw
// GGUF file into colibri's own model-directory format via `coli
// import` -- required once per model before ServeCommand can serve it
// (verified against the real v2.1.0 release: `coli serve --model`
// expects a colibri model directory, not a raw .gguf path).
func ImportCommand(pythonPath, launcherPath, ggufPath, outDir string) (prog string, args []string) {
	return pythonPath, []string{launcherPath, "import", ggufPath, "--out", outDir}
}

// ImportedModelDir returns where an already-imported colibri model
// directory for a given GGUF file should live, keyed by the GGUF's own
// filename so importing the same file twice is a cheap no-op (callers
// check for this directory before running ImportCommand).
func ImportedModelDir(colibriDir, ggufFileName string) string {
	return filepath.Join(colibriDir, "models", ggufFileName)
}

// RunImport runs `coli import` synchronously, converting ggufPath into
// a colibri model directory at outDir, streaming its stdout/stderr to
// the given writers (CLI output: this is a one-time, user-initiated,
// potentially long-running conversion for a very large model -- the
// same "visible, explicit, can take a while" contract `runyx pull`
// already has for model downloads, not something to run silently
// inside a request handler). Deliberately not wired into the live chat
// request path for exactly that reason: see cmd_gpu_saver.go's
// `runyx gpu-saver import` command, the only caller.
func RunImport(ctx context.Context, pythonPath, launcherPath, ggufPath, outDir string, stdout, stderr *os.File) error {
	prog, args := ImportCommand(pythonPath, launcherPath, ggufPath, outDir)
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("coli import: %w", err)
	}
	return nil
}

// ServeCommand returns the program and arguments to launch colibri's
// OpenAI-compatible server (`coli serve`) for an already-imported model
// directory. modelID is the client-facing model name (matches
// llama-server's own --model-id-equivalent usage in Runyx); apiKey, if
// non-empty, is required by every request to this instance (colibri's
// own --api-key flag, verified against api.md).
func ServeCommand(pythonPath, launcherPath, modelDir string, port int, modelID, apiKey string) (prog string, args []string) {
	args = []string{
		launcherPath, "serve",
		"--model", modelDir,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--model-id", modelID,
	}
	if apiKey != "" {
		args = append(args, "--api-key", apiKey)
	}
	return pythonPath, args
}
