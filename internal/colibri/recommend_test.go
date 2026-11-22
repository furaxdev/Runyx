package colibri

import (
	"testing"

	"github.com/furaxdev/runyx/internal/hwdetect"
)

func TestLooksLikeMoEMatchesKnownFamilies(t *testing.T) {
	for _, family := range []string{"GLM-5.2", "DeepSeek V4.1", "Kimi K3", "Qwen3.6-35B-A3B", "OLMoE", "a custom MoE build"} {
		if !LooksLikeMoE(family) {
			t.Errorf("LooksLikeMoE(%q) = false, want true", family)
		}
	}
}

func TestLooksLikeMoEDoesNotMatchDenseModels(t *testing.T) {
	for _, family := range []string{"Qwen2.5", "Llama-3.1", "Mistral-7B", "Phi-4"} {
		if LooksLikeMoE(family) {
			t.Errorf("LooksLikeMoE(%q) = true, want false (dense model)", family)
		}
	}
}

func TestRecommendedFalseForDenseModelRegardlessOfSize(t *testing.T) {
	info := hwdetect.Info{TotalRAMMB: 8 * 1024} // 8 GB RAM, no GPU
	huge := int64(100) * 1024 * 1024 * 1024     // 100 GB, far bigger than RAM
	rec := Recommended(info, huge, "Qwen2.5")
	if rec.Recommended {
		t.Errorf("expected no recommendation for a dense model regardless of size, got: %+v", rec)
	}
}

func TestRecommendedFalseWhenMoEModelFitsComfortably(t *testing.T) {
	info := hwdetect.Info{TotalRAMMB: 64 * 1024} // 64 GB RAM
	small := int64(20) * 1024 * 1024 * 1024      // 20 GB, comfortably under 64 GB
	rec := Recommended(info, small, "GLM-5.2")
	if rec.Recommended {
		t.Errorf("expected no recommendation when the model fits comfortably, got: %+v", rec)
	}
}

func TestRecommendedTrueForMoEModelExceedingFastMemory(t *testing.T) {
	info := hwdetect.Info{TotalRAMMB: 16 * 1024} // 16 GB RAM, no GPU
	huge := int64(400) * 1024 * 1024 * 1024      // 400 GB, far exceeds 16 GB
	rec := Recommended(info, huge, "GLM-5.2")
	if !rec.Recommended {
		t.Errorf("expected a recommendation for a huge MoE model on a 16GB machine, got: %+v", rec)
	}
	if rec.Reason == "" {
		t.Error("expected a non-empty reason")
	}
}

func TestRecommendedCountsVRAMTowardFastMemory(t *testing.T) {
	// A model that would need GPU Saver on RAM alone, but fits once a
	// large GPU's VRAM is counted too.
	modelSize := int64(40) * 1024 * 1024 * 1024 // 40 GB
	ramOnly := hwdetect.Info{TotalRAMMB: 16 * 1024}
	if rec := Recommended(ramOnly, modelSize, "GLM-5.2"); !rec.Recommended {
		t.Error("expected a recommendation when only counting 16GB RAM against a 40GB model")
	}
	withGPU := hwdetect.Info{TotalRAMMB: 16 * 1024, GPUs: []hwdetect.GPU{{VRAMMB: 48 * 1024}}}
	if rec := Recommended(withGPU, modelSize, "GLM-5.2"); rec.Recommended {
		t.Error("expected no recommendation once VRAM brings total fast memory well above the model size")
	}
}
