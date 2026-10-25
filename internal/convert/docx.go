package convert

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Minimal WordprocessingML model — just enough structure (paragraphs,
// run text, heading styles, tables) to render readable Markdown. Fields
// are matched by local name regardless of namespace prefix, since
// encoding/xml strips prefixes we don't register.

type wDocument struct {
	Body wBody `xml:"body"`
}

type wBody struct {
	Paragraphs []wParagraph `xml:"p"`
	Tables     []wTable     `xml:"tbl"`
	// Interleaved content is lost with this simple model (paragraphs and
	// tables are read as separate lists, not in original document order);
	// acceptable for a first pass at "keep structure", revisit if users
	// report tables landing in the wrong place.
}

type wParagraph struct {
	PPr  *wParagraphProps `xml:"pPr"`
	Runs []wRun           `xml:"r"`
}

type wParagraphProps struct {
	PStyle wStyleRef `xml:"pStyle"`
}

type wStyleRef struct {
	Val string `xml:"val,attr"`
}

func (p wParagraph) style() string {
	if p.PPr == nil {
		return ""
	}
	return p.PPr.PStyle.Val
}

type wRun struct {
	Text []string `xml:"t"`
}

type wTable struct {
	Rows []wRow `xml:"tr"`
}

type wRow struct {
	Cells []wCell `xml:"tc"`
}

type wCell struct {
	Paragraphs []wParagraph `xml:"p"`
}

func (p wParagraph) text() string {
	var b strings.Builder
	for _, r := range p.Runs {
		for _, t := range r.Text {
			b.WriteString(t)
		}
	}
	return b.String()
}

func convertDOCX(path, name string) (Result, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return Result{}, fmt.Errorf("open docx %s: %w", name, err)
	}
	defer r.Close()

	data, err := readZipMember(r, "word/document.xml")
	if err != nil {
		return Result{}, fmt.Errorf("docx %s: %w", name, err)
	}

	var doc wDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("parse docx %s: %w", name, err)
	}

	var b strings.Builder
	for _, p := range doc.Body.Paragraphs {
		text := strings.TrimSpace(p.text())
		if text == "" {
			continue
		}
		style := p.style()
		if strings.HasPrefix(style, "Heading") {
			level := 2
			if len(style) > len("Heading") {
				if n := style[len("Heading"):]; n >= "1" && n <= "6" {
					level = 1 + int(n[0]-'0')
				}
			}
			fmt.Fprintf(&b, "\n%s %s\n\n", strings.Repeat("#", level), text)
		} else {
			fmt.Fprintf(&b, "%s\n\n", text)
		}
	}
	for _, tbl := range doc.Body.Tables {
		b.WriteString("\n")
		b.WriteString(renderMarkdownTable(tbl))
		b.WriteString("\n")
	}

	return Result{FileName: name, Kind: "docx", Text: strings.TrimSpace(b.String())}, nil
}

func renderMarkdownTable(tbl wTable) string {
	var b strings.Builder
	for i, row := range tbl.Rows {
		cells := make([]string, len(row.Cells))
		for j, c := range row.Cells {
			var parts []string
			for _, p := range c.Paragraphs {
				if t := strings.TrimSpace(p.text()); t != "" {
					parts = append(parts, t)
				}
			}
			cells[j] = strings.Join(parts, " ")
		}
		fmt.Fprintf(&b, "| %s |\n", strings.Join(cells, " | "))
		if i == 0 {
			sep := make([]string, len(cells))
			for j := range sep {
				sep[j] = "---"
			}
			fmt.Fprintf(&b, "| %s |\n", strings.Join(sep, " | "))
		}
	}
	return b.String()
}

// maxZipMemberBytes bounds how much decompressed data readZipMember
// will ever read from one archive member, regardless of what the zip
// header claims that member's size is (attacker-controlled, so never
// trusted on its own). A zip/decompression bomb -- a crafted .docx/
// .xlsx/.pptx a few KB in size that decompresses to gigabytes -- would
// otherwise exhaust memory or hang the daemon on nothing more than a
// user uploading it as an attachment. 200MB is far beyond any real
// Office XML part this package will ever legitimately need to parse
// (even a very large document's XML is typically a few MB), so this
// never truncates a genuine file.
const maxZipMemberBytes = 200 << 20

func readZipMember(r *zip.ReadCloser, name string) ([]byte, error) {
	return readZipMemberLimited(r, name, maxZipMemberBytes)
}

// readZipMemberLimited is the actual implementation, parameterized on
// the byte limit so tests can exercise the bomb-refusal path with a
// small fixture instead of a real multi-hundred-MB one (slow to
// generate/compress/decompress even though it compresses well).
// readZipMember above is the only real caller and always passes
// maxZipMemberBytes.
func readZipMemberLimited(r *zip.ReadCloser, name string, limit int64) ([]byte, error) {
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			data, err := io.ReadAll(io.LimitReader(rc, limit+1))
			if err != nil {
				return nil, err
			}
			if int64(len(data)) > limit {
				return nil, fmt.Errorf("member %q exceeds %d bytes decompressed -- refusing (possible zip bomb)", name, limit)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("member %q not found in archive", name)
}
