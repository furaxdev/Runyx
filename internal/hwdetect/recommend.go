package hwdetect

import "sort"

// Quantization is a GGUF quantization level, ordered roughly by size (and
// quality) ascending.
type Quantization string

const (
	Q4KM Quantization = "Q4_K_M"
	Q5KM Quantization = "Q5_K_M"
	Q6K  Quantization = "Q6_K"
	Q8_0 Quantization = "Q8_0"
	F16  Quantization = "F16"
)

// bitsPerWeight approximates on-disk/VRAM bits-per-weight for each
// quantization, used to estimate whether a model fits.
var bitsPerWeight = map[Quantization]float64{
	Q4KM: 4.5,
	Q5KM: 5.5,
	Q6K:  6.6,
	Q8_0: 8.5,
	F16:  16.0,
}

// ModelCandidate is a model Runyx could recommend, before sizing it to the
// current machine.
type ModelCandidate struct {
	Name    string  // display name, e.g. "Llama 3.1 8B Instruct"
	Repo    string  // Hugging Face repo id
	ParamsB float64 // parameter count in billions
}

// Recommendation is a candidate sized to fit the detected hardware at the
// best quantization that still leaves headroom.
type Recommendation struct {
	ModelCandidate
	Quant      Quantization
	EstSizeMB  int
	FitsInVRAM bool // fully offloadable to GPU
	Headroom   string
}

// defaultCatalog is a small, curated list of well-known instruction models
// spanning sizes, used until live Hugging Face search (M1 `pull`) replaces
// it as the default recommendation source.
//
// Flagged, not removed: the default Q4_K_M pull of
// Qwen/Qwen2.5-0.5B-Instruct-GGUF was proven to produce garbled
// multilingual output (not a crash) in a 5-way CI isolation matrix —
// see docs/DECISIONS.md's "llama.cpp build pinning" entry's
// Correction. Not checked against this repo's other quantizations, so
// left in the catalog rather than guessed-removed; `runyx run <model>
// --selftest` is the fastest way to re-verify a given pull.
var defaultCatalog = []ModelCandidate{
	{Name: "Qwen2.5 0.5B Instruct", Repo: "Qwen/Qwen2.5-0.5B-Instruct-GGUF", ParamsB: 0.5},
	{Name: "Llama 3.2 3B Instruct", Repo: "meta-llama/Llama-3.2-3B-Instruct-GGUF", ParamsB: 3},
	{Name: "Qwen2.5 7B Instruct", Repo: "Qwen/Qwen2.5-7B-Instruct-GGUF", ParamsB: 7},
	{Name: "Llama 3.1 8B Instruct", Repo: "meta-llama/Llama-3.1-8B-Instruct-GGUF", ParamsB: 8},
	{Name: "Mixtral 8x7B Instruct", Repo: "mistralai/Mixtral-8x7B-Instruct-v0.1-GGUF", ParamsB: 46.7},
	{Name: "Llama 3.1 70B Instruct", Repo: "meta-llama/Llama-3.1-70B-Instruct-GGUF", ParamsB: 70},
}

// estimateSizeMB estimates a quantized model's memory footprint in MB.
func estimateSizeMB(paramsB float64, q Quantization) int {
	bits := bitsPerWeight[q]
	bytes := paramsB * 1e9 * bits / 8
	// llama.cpp context/KV-cache and runtime overhead; rough rule of thumb.
	const overheadMB = 512
	return int(bytes/(1024*1024)) + overheadMB
}

// Recommend picks, for each catalog model, the highest-quality quantization
// that fits the available memory budget (VRAM if a capable GPU is present,
// otherwise system RAM), and returns all that fit, best quality first per
// model, largest model first overall.
func Recommend(info Info) []Recommendation {
	budgetMB := info.TotalVRAMMB()
	usingGPU := budgetMB > 0 && info.BestBackend() != "cpu" && info.BestBackend() != "cpu-avx2"
	if !usingGPU {
		budgetMB = info.TotalRAMMB
	}
	// Leave headroom for the OS and the rest of the stack.
	usable := int(float64(budgetMB) * 0.8)

	quantsBestFirst := []Quantization{F16, Q8_0, Q6K, Q5KM, Q4KM}

	var out []Recommendation
	for _, cand := range defaultCatalog {
		var best *Recommendation
		for _, q := range quantsBestFirst {
			size := estimateSizeMB(cand.ParamsB, q)
			if size <= usable {
				best = &Recommendation{
					ModelCandidate: cand,
					Quant:          q,
					EstSizeMB:      size,
					FitsInVRAM:     usingGPU,
				}
				break
			}
		}
		if best == nil {
			continue // doesn't fit even at the smallest quantization
		}
		if usable > 0 {
			pct := 100 - int(float64(best.EstSizeMB)/float64(usable)*100)
			if pct < 0 {
				pct = 0
			}
			best.Headroom = headroomLabel(pct)
		}
		out = append(out, *best)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ParamsB > out[j].ParamsB
	})
	return out
}

// Fit is how comfortably a model of a known size should run on this
// machine, used by the Models page's "Runs well on your machine" /
// "Tight fit" / "Too large" badge. Unlike Recommend (which estimates a
// size from a parameter count and a chosen quantization), this takes
// the registry's actual, verified download size directly — no
// estimation needed.
type Fit string

const (
	FitComfortable Fit = "comfortable"
	FitOK          Fit = "ok"
	FitTight       Fit = "tight"
	FitTooLarge    Fit = "too_large"
)

// FitFor classifies sizeBytes against this machine's available budget
// (VRAM if a capable GPU is present, otherwise system RAM), the same
// 80%-of-budget headroom rule Recommend uses.
func FitFor(info Info, sizeBytes int64) Fit {
	budgetMB := info.TotalVRAMMB()
	usingGPU := budgetMB > 0 && info.BestBackend() != "cpu" && info.BestBackend() != "cpu-avx2"
	if !usingGPU {
		budgetMB = info.TotalRAMMB
	}
	usable := int64(float64(budgetMB) * 0.8 * 1024 * 1024) // MB -> bytes
	if usable <= 0 {
		return FitOK // unknown hardware budget; don't claim "too large" without evidence
	}
	if sizeBytes > usable {
		return FitTooLarge
	}
	pctFree := 100 - int(float64(sizeBytes)/float64(usable)*100)
	switch {
	case pctFree >= 50:
		return FitComfortable
	case pctFree >= 20:
		return FitOK
	default:
		return FitTight
	}
}

func headroomLabel(pctFree int) string {
	switch {
	case pctFree >= 50:
		return "comfortable"
	case pctFree >= 20:
		return "ok"
	default:
		return "tight"
	}
}

// TuneParams are the llama-server flags Runyx derived for a given model on
// this machine.
type TuneParams struct {
	NGPULayers int // -ngl; -1 means "offload everything"
	Threads    int // -t
	CtxSize    int // -c
	BatchSize  int // -b
	FlashAttn  bool
}

// Tune derives starting llama-server parameters for a model of the given
// size on the detected hardware. It's a heuristic starting point, not a
// replacement for `runyx bench`-driven auto-tuning (tracked for M6).
func Tune(info Info, estSizeMB int) TuneParams {
	p := TuneParams{
		Threads:   info.CPUThreads,
		CtxSize:   4096,
		BatchSize: 512,
		FlashAttn: info.HasAVX2 || len(info.GPUs) > 0,
	}
	vram := info.TotalVRAMMB()
	switch {
	case len(info.GPUs) == 0:
		p.NGPULayers = 0
	case vram >= int(float64(estSizeMB)*1.1):
		p.NGPULayers = -1 // fits entirely on GPU
	case vram > 0:
		// Partial offload: rough proportional split, refined later by
		// `runyx tune`'s benchmarking pass (M6).
		frac := float64(vram) / float64(estSizeMB)
		if frac > 1 {
			frac = 1
		}
		p.NGPULayers = int(frac * 32) // 32 is a common total-layer ballpark
	}
	return p
}
