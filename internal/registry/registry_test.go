package registry

import "testing"

func TestLoadParsesEmbeddedCatalog(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reg.Models) < 25 {
		t.Fatalf("got %d models, want at least 25", len(reg.Models))
	}

	seen := map[string]bool{}
	for _, m := range reg.Models {
		if m.ID == "" || m.Repo == "" || m.File == "" {
			t.Errorf("model missing required fields: %+v", m)
		}
		if seen[m.ID] {
			t.Errorf("duplicate model id %q", m.ID)
		}
		seen[m.ID] = true
		if m.SizeBytes <= 0 {
			t.Errorf("%s: size_bytes must be positive, got %d", m.ID, m.SizeBytes)
		}
		if m.PullSpec() != m.Repo+":"+m.File {
			t.Errorf("%s: PullSpec() = %q", m.ID, m.PullSpec())
		}
	}
}
