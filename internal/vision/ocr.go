// Package vision implements the vision bridge for non-vision models
// (FEATURES.md item 31): local OCR plus an optional small auxiliary
// vision model, so any text model can "see" an image even without
// native multimodal support.
package vision

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// OCR wraps the Tesseract CLI. Tesseract is a system dependency (not
// fetched by Runyx itself — unlike llama.cpp/whisper.cpp there is no
// single official prebuilt-binary release artifact per OS/arch): on
// Debian/Ubuntu `apt install tesseract-ocr`, on macOS `brew install
// tesseract`, on Windows the UB-Mannheim installer. OCR() degrades to a
// clear error (not a crash) when the binary isn't found, matching the
// "graceful degradation" requirement in item 31.
type OCR struct {
	// Languages passed to `tesseract -l <langs>`, e.g. "fra+eng".
	// Defaults to "eng" if empty.
	Languages string
}

// Available reports whether the tesseract binary is on PATH.
func Available() bool {
	_, err := exec.LookPath("tesseract")
	return err == nil
}

// Text runs OCR on an image file and returns the recognized text.
func (o OCR) Text(ctx context.Context, imagePath string) (string, error) {
	if !Available() {
		return "", fmt.Errorf("tesseract OCR is not installed (see docs/USAGE.md for install instructions); image analysis is unavailable without it or an auxiliary vision model")
	}
	langs := o.Languages
	if langs == "" {
		langs = "eng"
	}
	cmd := exec.CommandContext(ctx, "tesseract", imagePath, "stdout", "-l", langs)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tesseract failed: %w (%s)", err, lastLines(string(out), 5))
	}
	return strings.TrimSpace(string(out)), nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
