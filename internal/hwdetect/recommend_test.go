package hwdetect

import "testing"

func TestRecommendFitsSmallModelOnModestRAM(t *testing.T) {
	info := Info{TotalRAMMB: 8192} // 8GB RAM, no GPU
	recs := Recommend(info)
	if len(recs) == 0 {
		t.Fatal("expected at least one model to fit in 8GB RAM")
	}
	for _, r := range recs {
		if r.EstSizeMB > int(float64(info.TotalRAMMB)*0.8) {
			t.Errorf("recommendation %s (%d MB) exceeds usable budget", r.Name, r.EstSizeMB)
		}
	}
	// The 70B model must not appear on an 8GB machine at any quantization.
	for _, r := range recs {
		if r.ParamsB >= 70 {
			t.Errorf("70B model should not fit in 8GB RAM, got recommendation %+v", r)
		}
	}
}

func TestRecommendOrdersLargestFirst(t *testing.T) {
	info := Info{TotalRAMMB: 256 * 1024} // 256GB, everything fits
	recs := Recommend(info)
	for i := 1; i < len(recs); i++ {
		if recs[i].ParamsB > recs[i-1].ParamsB {
			t.Errorf("recommendations not sorted largest-first: %v then %v", recs[i-1].Name, recs[i].Name)
		}
	}
}

func TestBestBackendPriority(t *testing.T) {
	info := Info{
		HasAVX2: true,
		GPUs: []GPU{
			{Backend: "vulkan"},
			{Backend: "cuda"},
		},
	}
	if got := info.BestBackend(); got != "cuda" {
		t.Errorf("BestBackend() = %q, want cuda", got)
	}
}

func TestBestBackendFallsBackToCPU(t *testing.T) {
	info := Info{HasAVX2: true}
	if got := info.BestBackend(); got != "cpu-avx2" {
		t.Errorf("BestBackend() = %q, want cpu-avx2", got)
	}
	info2 := Info{}
	if got := info2.BestBackend(); got != "cpu" {
		t.Errorf("BestBackend() = %q, want cpu", got)
	}
}

func TestTuneFullOffloadWhenVRAMFits(t *testing.T) {
	info := Info{CPUThreads: 8, GPUs: []GPU{{VRAMMB: 24000, Backend: "cuda"}}}
	p := Tune(info, 8000)
	if p.NGPULayers != -1 {
		t.Errorf("NGPULayers = %d, want -1 (full offload)", p.NGPULayers)
	}
}

func TestTuneCPUOnlyWhenNoGPU(t *testing.T) {
	info := Info{CPUThreads: 4}
	p := Tune(info, 4000)
	if p.NGPULayers != 0 {
		t.Errorf("NGPULayers = %d, want 0 on CPU-only machine", p.NGPULayers)
	}
	if p.Threads != 4 {
		t.Errorf("Threads = %d, want 4", p.Threads)
	}
}

func TestFitForClassification(t *testing.T) {
	info := Info{TotalRAMMB: 16384} // 16GB RAM, no GPU
	cases := []struct {
		sizeBytes int64
		want      Fit
	}{
		{500 * 1024 * 1024, FitComfortable},           // 500MB, tiny
		{int64(16384) * 1024 * 1024 * 2, FitTooLarge}, // 2x total RAM
	}
	for _, c := range cases {
		if got := FitFor(info, c.sizeBytes); got != c.want {
			t.Errorf("FitFor(%d bytes) = %q, want %q", c.sizeBytes, got, c.want)
		}
	}
}

func TestFitForUnknownBudgetNeverClaimsTooLarge(t *testing.T) {
	info := Info{} // nothing detected
	if got := FitFor(info, 50*1024*1024*1024); got == FitTooLarge {
		t.Errorf("FitFor with unknown budget = %q, should not claim too_large without evidence", got)
	}
}
