package convert

import (
	"strings"
	"testing"
)

func TestConvertPDF(t *testing.T) {
	res, err := File("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if res.Kind != "pdf" {
		t.Errorf("Kind = %q, want pdf", res.Kind)
	}
	if !strings.Contains(res.Text, "Bonjour Runyx") {
		t.Errorf("page 1 text missing: %q", res.Text)
	}
	if !strings.Contains(res.Text, "Deuxieme page") {
		t.Errorf("page 2 text missing: %q", res.Text)
	}
	if !strings.Contains(res.Text, "## Page 1") || !strings.Contains(res.Text, "## Page 2") {
		t.Errorf("page headings missing: %q", res.Text)
	}
}
