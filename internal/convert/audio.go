package convert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/furaxdev/runyx/internal/voice"
)

// AudioToText transcribes an audio attachment using a local whisper.cpp
// instance, completing the "audio via local transcription" part of
// item 17.
func AudioToText(ctx context.Context, tr *voice.Transcriber, path string) (Result, error) {
	name := filepath.Base(path)
	wavPath, cleanup, err := toWAV(ctx, path)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()

	t, err := tr.Transcribe(ctx, wavPath)
	if err != nil {
		return Result{}, fmt.Errorf("transcribe %s: %w", name, err)
	}
	var warnings []string
	if t.Language != "" {
		warnings = append(warnings, fmt.Sprintf("detected language: %s", t.Language))
	}
	return Result{FileName: name, Kind: "audio", Text: t.Text, Warnings: warnings}, nil
}

// VideoToText extracts the audio track and transcribes it (video
// keyframe extraction for the vision bridge, item 31, is handled
// separately by internal/vision — this covers the "transcription"
// half of item 17's video support).
func VideoToText(ctx context.Context, tr *voice.Transcriber, path string) (Result, error) {
	res, err := AudioToText(ctx, tr, path)
	if err != nil {
		return Result{}, err
	}
	res.Kind = "video"
	res.Warnings = append(res.Warnings, "keyframe analysis not included — see internal/vision for image understanding")
	return res, nil
}

func toWAV(ctx context.Context, path string) (wavPath string, cleanup func(), err error) {
	tmp, err := os.CreateTemp("", "runyx-audio-*.wav")
	if err != nil {
		return "", nil, err
	}
	tmp.Close()
	if err := voice.ConvertToWAV(ctx, path, tmp.Name()); err != nil {
		os.Remove(tmp.Name())
		return "", nil, err
	}
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}
