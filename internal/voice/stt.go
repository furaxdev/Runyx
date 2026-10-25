package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Transcriber wraps a whisper.cpp "whisper-cli" binary to turn audio
// into text. whisper-cli (built with --output-json) prints a JSON
// transcript with per-segment timestamps and auto-detects the spoken
// language, which is how FR/EN auto-detection (item 25) is satisfied
// without any extra logic here.
type Transcriber struct {
	BinaryPath string
	ModelPath  string // path to a whisper GGML/GGUF model, e.g. ggml-base.bin
}

// Segment is one transcribed utterance with its timing.
type Segment struct {
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
	Text    string `json:"text"`
}

// Transcription is the full result of transcribing one audio file.
type Transcription struct {
	Language string    `json:"language"`
	Segments []Segment `json:"segments"`
	Text     string    `json:"text"`
}

// whisperJSON mirrors whisper.cpp's --output-json shape closely enough
// to extract what Runyx needs (documented in its README; not copied
// from source).
type whisperJSON struct {
	Result struct {
		Language string `json:"language"`
	} `json:"result"`
	Transcription []struct {
		Timestamps struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"timestamps"`
		Offsets struct {
			From int64 `json:"from"`
			To   int64 `json:"to"`
		} `json:"offsets"`
		Text string `json:"text"`
	} `json:"transcription"`
}

// Transcribe runs whisper-cli on an audio file (any format ffmpeg can
// decode; whisper.cpp itself expects 16kHz mono WAV, so callers should
// pre-convert with ffmpeg — see ConvertToWAV) and returns the parsed
// transcript.
func (t *Transcriber) Transcribe(ctx context.Context, audioWAVPath string) (Transcription, error) {
	if t.BinaryPath == "" {
		return Transcription{}, fmt.Errorf("whisper binary not available; run `runyx doctor` to fetch it")
	}
	outBase := audioWAVPath // whisper-cli appends .json to -of base name
	args := []string{
		"-m", t.ModelPath,
		"-f", audioWAVPath,
		"--output-json",
		"-of", outBase,
		"--no-prints",
	}
	cmd := exec.CommandContext(ctx, t.BinaryPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Transcription{}, fmt.Errorf("whisper-cli failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	defer os.Remove(outBase + ".json")

	data, err := os.ReadFile(outBase + ".json")
	if err != nil {
		return Transcription{}, fmt.Errorf("read whisper output: %w", err)
	}
	var wj whisperJSON
	if err := json.Unmarshal(data, &wj); err != nil {
		return Transcription{}, fmt.Errorf("parse whisper output: %w", err)
	}

	var b strings.Builder
	result := Transcription{Language: wj.Result.Language}
	for _, seg := range wj.Transcription {
		text := strings.TrimSpace(seg.Text)
		result.Segments = append(result.Segments, Segment{StartMS: seg.Offsets.From, EndMS: seg.Offsets.To, Text: text})
		b.WriteString(text)
		b.WriteString(" ")
	}
	result.Text = strings.TrimSpace(b.String())
	return result, nil
}

// ConvertToWAV uses ffmpeg (must be on PATH) to convert any audio/video
// file to the 16kHz mono WAV whisper.cpp expects.
func ConvertToWAV(ctx context.Context, inputPath, outputWAVPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found on PATH (required to decode %s for transcription): %w", filepath.Base(inputPath), err)
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", inputPath, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", outputWAVPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w (%s)", err, lastLines(string(out), 5))
	}
	return nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
