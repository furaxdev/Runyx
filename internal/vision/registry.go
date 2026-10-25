package vision

// AuxiliaryModel is one recommended small vision model Runyx can pull
// and run as the vision auxiliary for text-only models. Kept as a small
// maintained registry per item 31 ("pick and maintain a registry of
// recommended vision models").
type AuxiliaryModel struct {
	Name       string
	Repo       string // Hugging Face repo with the GGUF + mmproj files
	MMProjFile string // multimodal projector filename within Repo
	ModelFile  string // preferred quantized model filename within Repo
	ParamsB    float64
}

// Recommended lists small vision models suitable as the auxiliary,
// smallest first so `recommend` can pick the first that fits.
var Recommended = []AuxiliaryModel{
	{
		Name:       "Moondream2",
		Repo:       "vikhyatk/moondream2",
		ParamsB:    1.9,
		ModelFile:  "moondream2-text-model-f16.gguf",
		MMProjFile: "moondream2-mmproj-f16.gguf",
	},
	{
		Name:       "SmolVLM-256M-Instruct",
		Repo:       "HuggingFaceTB/SmolVLM-256M-Instruct-GGUF",
		ParamsB:    0.256,
		ModelFile:  "SmolVLM-256M-Instruct-Q8_0.gguf",
		MMProjFile: "mmproj-SmolVLM-256M-Instruct-Q8_0.gguf",
	},
	{
		Name:       "Qwen2-VL-2B-Instruct",
		Repo:       "Qwen/Qwen2-VL-2B-Instruct-GGUF",
		ParamsB:    2,
		ModelFile:  "qwen2-vl-2b-instruct-q4_k_m.gguf",
		MMProjFile: "qwen2-vl-2b-instruct-vision.gguf",
	},
}

// PickForRAM returns the largest recommended model whose rough memory
// footprint (≈1 GB per billion parameters at Q4-ish quantization, plus
// overhead) fits within availableMB, or the smallest model if none
// comfortably fit (better a slow answer than none).
func PickForRAM(availableMB int) AuxiliaryModel {
	best := Recommended[len(Recommended)-1] // smallest, as a safe fallback
	for _, m := range Recommended {
		estMB := int(m.ParamsB*1024) + 512
		if estMB <= availableMB {
			return m
		}
	}
	return best
}
