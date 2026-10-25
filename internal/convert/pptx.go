package convert

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type pTextBody struct {
	Paragraphs []pParagraph `xml:"p"`
}

type pParagraph struct {
	Runs []pRun `xml:"r"`
}

type pRun struct {
	Text string `xml:"t"`
}

type pSlide struct {
	TextBodies []pTextBody `xml:"cSld>spTree>sp>txBody"`
}

// convertPPTX renders each slide's text frames as a Markdown section,
// one heading per slide, in slide order.
func convertPPTX(path, name string) (Result, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return Result{}, fmt.Errorf("open pptx %s: %w", name, err)
	}
	defer r.Close()

	type slideFile struct {
		num  int
		name string
	}
	var slides []slideFile
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			numStr := strings.TrimSuffix(strings.TrimPrefix(f.Name, "ppt/slides/slide"), ".xml")
			n, _ := strconv.Atoi(numStr)
			slides = append(slides, slideFile{num: n, name: f.Name})
		}
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].num < slides[j].num })

	var b strings.Builder
	var warnings []string
	for _, sf := range slides {
		data, err := readZipMember(r, sf.name)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		var slide pSlide
		if err := xml.Unmarshal(data, &slide); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", sf.name, err))
			continue
		}
		fmt.Fprintf(&b, "## Slide %d\n\n", sf.num)
		for _, tb := range slide.TextBodies {
			for _, p := range tb.Paragraphs {
				var line strings.Builder
				for _, r := range p.Runs {
					line.WriteString(r.Text)
				}
				if strings.TrimSpace(line.String()) != "" {
					fmt.Fprintf(&b, "%s\n", line.String())
				}
			}
		}
		b.WriteString("\n")
	}

	return Result{FileName: name, Kind: "pptx", Text: strings.TrimSpace(b.String()), Warnings: warnings}, nil
}
