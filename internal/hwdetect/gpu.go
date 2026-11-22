package hwdetect

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// gpuProbeTimeout bounds every GPU-vendor-tool subprocess call below
// (nvidia-smi, rocm-smi, sysctl). A real, responding tool answers in
// well under a second; this exists so a stalled driver or a tool that
// hangs instead of erroring out can never block Detect() -- and
// therefore daemon.New(), and therefore the whole daemon/GUI startup
// path -- for an unbounded amount of time. Found the hard way: a CI
// runner where nvidia-smi exists but doesn't return made
// TestEnsureDaemonRunningFailsFastWhenPortInUse take 30+ seconds
// instead of failing fast, since daemon.New() (called before that
// test's own race-the-readiness-poll logic even starts) was the thing
// actually stuck.
const gpuProbeTimeout = 3 * time.Second

// detectGPUs probes for GPUs using whatever vendor tooling is present on
// PATH. This is best-effort: on a machine with no GPU, or none of these
// tools installed, it returns an empty slice and callers fall back to CPU.
func detectGPUs() []GPU {
	var gpus []GPU
	gpus = append(gpus, detectNVIDIA()...)
	gpus = append(gpus, detectAMD()...)
	if runtime.GOOS == "darwin" {
		gpus = append(gpus, detectAppleSilicon()...)
	}
	return gpus
}

func detectNVIDIA() []GPU {
	ctx, cancel := context.WithTimeout(context.Background(), gpuProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil
	}
	var gpus []GPU
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		vramMB, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		gpus = append(gpus, GPU{Name: name, VendorID: "nvidia", VRAMMB: vramMB, Backend: "cuda"})
	}
	return gpus
}

func detectAMD() []GPU {
	ctx, cancel := context.WithTimeout(context.Background(), gpuProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "rocm-smi", "--showproductname", "--showmeminfo", "vram", "--csv").Output()
	if err != nil {
		return nil
	}
	// rocm-smi's CSV output varies by version; best-effort: at minimum
	// report that a ROCm-capable GPU exists so the backend can be picked.
	if strings.TrimSpace(string(out)) == "" {
		return nil
	}
	return []GPU{{Name: "AMD GPU", VendorID: "amd", Backend: "rocm"}}
}

func detectAppleSilicon() []GPU {
	ctx, cancel := context.WithTimeout(context.Background(), gpuProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return nil
	}
	totalBytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return nil
	}
	// Apple Silicon shares system RAM with the GPU; report the whole pool
	// as "VRAM" since that's the practical ceiling for a loaded model.
	return []GPU{{Name: "Apple GPU", VendorID: "apple", VRAMMB: int(totalBytes / (1024 * 1024)), Backend: "metal"}}
}
