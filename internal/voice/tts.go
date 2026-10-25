package voice

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Synthesizer wraps a Piper binary to turn text into speech, writing a
// WAV file. Piper's CLI contract (documented at
// https://github.com/rhasspy/piper): text on stdin, "--model <onnx>
// --output_file <wav>" as flags.
type Synthesizer struct {
	BinaryPath string
	ModelPath  string // path to a Piper .onnx voice model
}

// Synthesize writes spoken audio for text to outputWAVPath.
func (s *Synthesizer) Synthesize(ctx context.Context, text, outputWAVPath string) error {
	if s.BinaryPath == "" {
		return fmt.Errorf("piper binary not available; run `runyx doctor` to fetch it")
	}
	cmd := exec.CommandContext(ctx, s.BinaryPath, "--model", s.ModelPath, "--output_file", outputWAVPath)
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("piper failed: %w (%s)", err, lastLines(string(out), 5))
	}
	return nil
}
