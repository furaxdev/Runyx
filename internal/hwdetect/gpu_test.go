package hwdetect

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFakeSlowBinary creates an executable at dir/name that sleeps
// well past gpuProbeTimeout before ever answering, simulating a
// vendor tool that hangs instead of erroring (a stalled driver, a
// broken sandbox/container shim, etc.) -- the real failure mode this
// guards against, found via a CI runner that behaved exactly this way.
func writeFakeSlowBinary(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	// "exec" replaces the shell's own process image with sleep (an
	// execve, not fork+exec) rather than running it as a child: a child
	// `sleep` would inherit the shell's stdout pipe and keep it open
	// for the full 30s even after exec.CommandContext kills the shell
	// on timeout, since killing a process doesn't kill its own
	// children. A real hung vendor tool is a single process, not a
	// shell with an orphaned grandchild -- "exec" here is what makes
	// this fixture behave like that real case instead of like neither.
	script := "#!/bin/sh\nexec sleep 30\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary %s: %v", name, err)
	}
}

// putFirstOnPath prepends dir to PATH for the duration of the test, so
// exec.Command's PATH lookup finds the fake binary before any real one.
func putFirstOnPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertBounded(t *testing.T, name string, fn func() []GPU) {
	t.Helper()
	start := time.Now()
	fn()
	elapsed := time.Since(start)
	// Generous margin above gpuProbeTimeout for scheduler/CI jitter --
	// the point is "nowhere near the 30s the fake binary sleeps for",
	// not a tight budget.
	if max := gpuProbeTimeout + 5*time.Second; elapsed > max {
		t.Errorf("%s took %s with a hanging binary on PATH, want well under %s (gpuProbeTimeout=%s)", name, elapsed, max, gpuProbeTimeout)
	}
}

func TestDetectNVIDIATimesOutAgainstAHangingBinary(t *testing.T) {
	dir := t.TempDir()
	writeFakeSlowBinary(t, dir, "nvidia-smi")
	putFirstOnPath(t, dir)
	assertBounded(t, "detectNVIDIA", detectNVIDIA)
}

func TestDetectAMDTimesOutAgainstAHangingBinary(t *testing.T) {
	dir := t.TempDir()
	writeFakeSlowBinary(t, dir, "rocm-smi")
	putFirstOnPath(t, dir)
	assertBounded(t, "detectAMD", detectAMD)
}

func TestDetectAppleSiliconTimesOutAgainstAHangingBinary(t *testing.T) {
	dir := t.TempDir()
	writeFakeSlowBinary(t, dir, "sysctl")
	putFirstOnPath(t, dir)
	assertBounded(t, "detectAppleSilicon", detectAppleSilicon)
}

// TestDetectGPUsNeverBlocksLongerThanAllProbesCombined guards the
// realistic worst case end to end: every vendor tool present but
// hanging must still bound the whole detectGPUs() call, not just each
// probe individually.
func TestDetectGPUsNeverBlocksLongerThanAllProbesCombined(t *testing.T) {
	dir := t.TempDir()
	writeFakeSlowBinary(t, dir, "nvidia-smi")
	writeFakeSlowBinary(t, dir, "rocm-smi")
	writeFakeSlowBinary(t, dir, "sysctl")
	putFirstOnPath(t, dir)

	start := time.Now()
	detectGPUs()
	elapsed := time.Since(start)

	// detectGPUs runs its probes sequentially, so the worst case is the
	// sum of each one's timeout (sysctl is only tried on darwin, but the
	// margin below covers that case too).
	if max := 3*gpuProbeTimeout + 5*time.Second; elapsed > max {
		t.Errorf("detectGPUs took %s with every vendor tool hanging, want well under %s", elapsed, max)
	}
}
