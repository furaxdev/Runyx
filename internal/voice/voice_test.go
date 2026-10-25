package voice

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// writeFakeScript writes an executable shell script standing in for a
// real whisper-cli/piper binary, so the subprocess plumbing (argument
// passing, stdin, output file handling) is exercised for real without
// needing a network download of the actual binary.
func writeFakeScript(t *testing.T, dir, name, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell-script binaries are POSIX-only; real binaries are tested manually on Windows (see docs/MANUAL_TESTS.md)")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTranscribeParsesWhisperJSON(t *testing.T) {
	dir := t.TempDir()
	// The fake binary ignores its real args and just writes the JSON
	// file whisper-cli would have produced at "<audio path>.json".
	script := `
for arg in "$@"; do
  :
done
# find the -of argument's value (basename for output), 2 args after -f
of=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-of" ]; then of="$a"; fi
  prev="$a"
done
cat > "${of}.json" <<'EOF'
{"result":{"language":"fr"},"transcription":[{"offsets":{"from":0,"to":1200},"text":" Bonjour tout le monde"}]}
EOF
`
	bin := writeFakeScript(t, dir, "whisper-cli", script)

	audioPath := filepath.Join(dir, "sample.wav")
	os.WriteFile(audioPath, []byte("fake-wav"), 0o644)

	tr := &Transcriber{BinaryPath: bin, ModelPath: "ggml-base.bin"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := tr.Transcribe(ctx, audioPath)
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if result.Language != "fr" {
		t.Errorf("Language = %q, want fr", result.Language)
	}
	if result.Text != "Bonjour tout le monde" {
		t.Errorf("Text = %q", result.Text)
	}
	if len(result.Segments) != 1 || result.Segments[0].EndMS != 1200 {
		t.Errorf("Segments = %+v", result.Segments)
	}
}

func TestTranscribeMissingBinary(t *testing.T) {
	tr := &Transcriber{}
	if _, err := tr.Transcribe(context.Background(), "whatever.wav"); err == nil {
		t.Error("expected error when BinaryPath is empty")
	}
}

func TestSynthesizeWritesOutputFile(t *testing.T) {
	dir := t.TempDir()
	script := `
prev=""
out=""
for a in "$@"; do
  if [ "$prev" = "--output_file" ]; then out="$a"; fi
  prev="$a"
done
cat > "$out" <<'EOF'
FAKE-WAV-BYTES
EOF
`
	bin := writeFakeScript(t, dir, "piper", script)
	outPath := filepath.Join(dir, "speech.wav")

	syn := &Synthesizer{BinaryPath: bin, ModelPath: "voice.onnx"}
	if err := syn.Synthesize(context.Background(), "Bonjour", outPath); err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty synthesized output")
	}
}

func TestSynthesizeMissingBinary(t *testing.T) {
	syn := &Synthesizer{}
	if err := syn.Synthesize(context.Background(), "hi", "/tmp/x.wav"); err == nil {
		t.Error("expected error when BinaryPath is empty")
	}
}

func TestConvertToWAVWithRealFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available in this environment")
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "tone.wav")
	// Generate a one-second test tone with ffmpeg's own lavfi source,
	// so this test has a real audio file to convert without needing any
	// network access.
	gen := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", input)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate test tone: %v (%s)", err, out)
	}

	output := filepath.Join(dir, "out16k.wav")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := ConvertToWAV(ctx, input, output); err != nil {
		t.Fatalf("ConvertToWAV: %v", err)
	}
	info, err := os.Stat(output)
	if err != nil || info.Size() == 0 {
		t.Fatalf("expected non-empty converted WAV, stat err=%v", err)
	}
}

func TestPickAssetPrefersMostSpecific(t *testing.T) {
	assets := []ghAsset{
		{Name: "whisper-bin-linux-x64.zip"},
		{Name: "whisper-bin-ubuntu-x64-cuda.zip"},
	}
	got, err := PickAsset(assets, []string{"ubuntu", "x64", "cuda"}, []string{"linux", "x64"})
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "whisper-bin-ubuntu-x64-cuda.zip" {
		t.Errorf("got %q, want the more specific match", got.Name)
	}
}

func TestPickAssetFallsBackToLessSpecific(t *testing.T) {
	assets := []ghAsset{{Name: "whisper-bin-linux-x64.zip"}}
	got, err := PickAsset(assets, []string{"ubuntu", "x64", "cuda"}, []string{"linux", "x64"})
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "whisper-bin-linux-x64.zip" {
		t.Errorf("got %q", got.Name)
	}
}
