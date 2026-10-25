package convert

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/furaxdev/runyx/internal/voice"
)

func TestAudioToTextWithFakeWhisper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell-script binaries are POSIX-only")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()

	input := filepath.Join(dir, "clip.wav")
	gen := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=220:duration=1", input)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate test audio: %v (%s)", err, out)
	}

	script := `#!/bin/sh
prev=""
of=""
for a in "$@"; do
  if [ "$prev" = "-of" ]; then of="$a"; fi
  prev="$a"
done
cat > "${of}.json" <<'EOF'
{"result":{"language":"en"},"transcription":[{"offsets":{"from":0,"to":900},"text":" This is a test clip"}]}
EOF
`
	binPath := filepath.Join(dir, "whisper-cli")
	os.WriteFile(binPath, []byte(script), 0o755)

	tr := &voice.Transcriber{BinaryPath: binPath, ModelPath: "ggml-tiny.bin"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := AudioToText(ctx, tr, input)
	if err != nil {
		t.Fatalf("AudioToText: %v", err)
	}
	if res.Kind != "audio" {
		t.Errorf("Kind = %q", res.Kind)
	}
	if res.Text != "This is a test clip" {
		t.Errorf("Text = %q", res.Text)
	}
}

func TestFileReturnsNeedsTranscriptionForAudioVideo(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".mp3", ".mp4"} {
		path := filepath.Join(dir, "clip"+ext)
		os.WriteFile(path, []byte("fake"), 0o644)
		_, err := File(path)
		if err == nil || !NeedsTranscription(err) {
			t.Errorf("File(%s): expected NeedsTranscription error, got %v", ext, err)
		}
	}
}
