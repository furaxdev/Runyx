// Package hwdetect inspects the local machine (CPU, RAM, GPU, VRAM) and
// turns that into model recommendations and llama-server tuning parameters.
// It powers `runyx doctor`, `runyx recommend`, and `runyx tune`.
package hwdetect

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// GPU describes one detected graphics accelerator.
type GPU struct {
	Name     string
	VendorID string // "nvidia", "amd", "apple", "intel", "unknown"
	VRAMMB   int    // 0 if unknown
	Backend  string // preferred llama.cpp backend: "cuda", "rocm", "vulkan", "metal"
}

// Info is a snapshot of the machine's compute resources.
type Info struct {
	OS         string
	Arch       string
	CPUModel   string
	CPUThreads int
	HasAVX2    bool
	TotalRAMMB int
	GPUs       []GPU
	// ggml ISA extensions beyond AVX2 that matter for CPU builds.
	HasAVX512 bool
}

// TotalVRAMMB sums VRAM across all detected GPUs.
func (i Info) TotalVRAMMB() int {
	total := 0
	for _, g := range i.GPUs {
		total += g.VRAMMB
	}
	return total
}

// BestBackend returns the llama.cpp backend variant Runyx should fetch and
// run with, in priority order: CUDA > ROCm > Metal > Vulkan > CPU.
func (i Info) BestBackend() string {
	for _, g := range i.GPUs {
		if g.Backend == "cuda" {
			return "cuda"
		}
	}
	for _, g := range i.GPUs {
		if g.Backend == "rocm" {
			return "rocm"
		}
	}
	for _, g := range i.GPUs {
		if g.Backend == "metal" {
			return "metal"
		}
	}
	for _, g := range i.GPUs {
		if g.Backend == "vulkan" {
			return "vulkan"
		}
	}
	if i.HasAVX2 {
		return "cpu-avx2"
	}
	return "cpu"
}

// Detect gathers hardware info for the current machine. It never fails
// hard: on any detection error for an optional field, it leaves that field
// at its zero value rather than returning an error, since `doctor` should
// always produce *something* useful.
func Detect() Info {
	info := Info{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		CPUThreads: runtime.NumCPU(),
	}
	detectCPU(&info)
	detectRAM(&info)
	info.GPUs = detectGPUs()
	return info
}

func detectCPU(info *Info) {
	if runtime.GOOS != "linux" {
		return
	}
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "model name"):
			if info.CPUModel == "" {
				info.CPUModel = valueAfterColon(line)
			}
		case strings.HasPrefix(line, "flags"):
			flags := valueAfterColon(line)
			info.HasAVX2 = strings.Contains(flags, "avx2")
			info.HasAVX512 = strings.Contains(flags, "avx512f")
			return // flags appear once per core block; first is enough
		}
	}
}

func detectRAM(info *Info) {
	if runtime.GOOS != "linux" {
		return
	}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kb, err := strconv.Atoi(fields[1]); err == nil {
					info.TotalRAMMB = kb / 1024
				}
			}
			return
		}
	}
}

func valueAfterColon(line string) string {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(line[idx+1:])
}
