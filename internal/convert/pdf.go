package convert

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// convertPDF extracts text page by page, keeping page boundaries as
// Markdown headings so citations (item 16) can point at a page number.
func convertPDF(path, name string) (Result, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return Result{}, fmt.Errorf("open pdf %s: %w", name, err)
	}
	defer f.Close()

	var b strings.Builder
	var warnings []string
	numPages := r.NumPage()
	for i := 1; i <= numPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("page %d: %v", i, err))
			continue
		}
		fmt.Fprintf(&b, "## Page %d\n\n%s\n\n", i, strings.TrimSpace(text))
	}

	if b.Len() == 0 {
		warnings = append(warnings, "no extractable text found (the PDF may be scanned images — OCR is not yet wired into this path, see docs/BLOCKED.md)")
	}

	return Result{FileName: name, Kind: "pdf", Text: b.String(), Warnings: warnings}, nil
}
