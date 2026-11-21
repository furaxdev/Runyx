package colibri

import (
	"fmt"
	"strings"

	"github.com/furaxdev/runyx/internal/hwdetect"
)

// knownMoEFamilies lists the model families colibri's own README
// documents as the MoE engines it targets -- lowercased substrings
// matched against a model's family/name string. This is a heuristic,
// not a real GGUF-metadata check: Runyx doesn't parse a GGUF's
// architecture/expert_count fields yet (a natural follow-up, not done
// here -- see docs/ROADMAP.md). colibri's whole advantage is specific
// to MoE architectures, where only a fraction of expert weights
// activate per token; recommending it for a dense model of any size
// would be actively wrong, since llama.cpp's ordinary partial GPU
// offload already handles "big dense model, some layers on CPU"
// reasonably, and colibri's disk-streaming expert tier has no
// corresponding benefit there.
var knownMoEFamilies = []string{
	"glm", "mistral-large", "kimi", "deepseek", "mimo", "qwen3.6", "qwen3.8", "olmoe", "inkling", "moe",
}

// LooksLikeMoE reports whether family (e.g. "GLM-5.2" or "Qwen2.5", the
// same strings used by internal/registry) names-match a known
// colibri-supported MoE family. See knownMoEFamilies's doc comment for
// the honest limits of this check.
func LooksLikeMoE(family string) bool {
	f := strings.ToLower(family)
	for _, known := range knownMoEFamilies {
		if strings.Contains(f, known) {
			return true
		}
	}
	return false
}

// fastMemHeadroom is how much a model's size must exceed a machine's
// combined RAM+VRAM before colibri is recommended over llama-server. A
// model that merely fits, even snugly, is still better served by
// llama-server's own GPU offload -- colibri's disk-streamed expert
// tier trades speed for being able to run at all, a trade only worth
// making once that's the actual alternative.
const fastMemHeadroom = 1.3

// Recommendation is Recommended's result: whether GPU Saver is likely
// to help, and a human-readable reason -- shown to the user (see the
// web Settings "About" section) rather than silently decided for them.
type Recommendation struct {
	Recommended bool
	Reason      string
}

// Recommended decides whether colibri (GPU Saver / CPU Saver) is
// likely to help running a model of modelSizeBytes, named/familied
// family, on a machine described by info. Both conditions must hold:
// family looks like one of the MoE families colibri actually targets
// (LooksLikeMoE), and the model doesn't comfortably fit in the
// machine's combined RAM+VRAM (fastMemHeadroom margin).
func Recommended(info hwdetect.Info, modelSizeBytes int64, family string) Recommendation {
	if !LooksLikeMoE(family) {
		return Recommendation{
			Recommended: false,
			Reason:      "not a known MoE model family; colibri's disk-tiered experts offer no benefit for a dense model",
		}
	}
	fastMemBytes := int64(info.TotalRAMMB+info.TotalVRAMMB()) * 1024 * 1024
	if fastMemBytes <= 0 || float64(modelSizeBytes) < float64(fastMemBytes)*fastMemHeadroom {
		return Recommendation{
			Recommended: false,
			Reason:      "model fits comfortably in available RAM/VRAM; llama-server's own GPU offload is simpler",
		}
	}
	return Recommendation{
		Recommended: true,
		Reason: fmt.Sprintf(
			"model is a known MoE family and is %.0f%% of available RAM+VRAM; colibri streams unused experts from disk instead of requiring the whole model resident",
			float64(modelSizeBytes)/float64(fastMemBytes)*100,
		),
	}
}
