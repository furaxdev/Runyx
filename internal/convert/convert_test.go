package convert

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConvertText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	os.WriteFile(path, []byte("# Titre\n\nContenu."), 0o644)
	res, err := File(path)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if res.Kind != "text" || !strings.Contains(res.Text, "Contenu.") {
		t.Errorf("got %+v", res)
	}
}

func TestConvertCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n"), 0o644)
	res, err := File(path)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if res.Kind != "code" {
		t.Errorf("Kind = %q, want code", res.Kind)
	}
}

func TestConvertArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.zip")
	writeZip(t, path, map[string]string{
		"readme.txt":  "Hello from the archive",
		"src/main.go": "package main",
		"image.bin":   "\x00\x01\x02binary-not-inlined",
	})
	res, err := File(path)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !strings.Contains(res.Text, "Hello from the archive") {
		t.Error("expected readme.txt content inlined")
	}
	if !strings.Contains(res.Text, "package main") {
		t.Error("expected main.go content inlined")
	}
	if strings.Contains(res.Text, "binary-not-inlined") {
		t.Error("binary file should not be inlined")
	}
}

func TestConvertDOCX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.docx")
	documentXML := `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Titre du document</w:t></w:r></w:p>
    <w:p><w:r><w:t>Un paragraphe normal.</w:t></w:r></w:p>
    <w:tbl>
      <w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc></w:tr>
      <w:tr><w:tc><w:p><w:r><w:t>1</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>2</w:t></w:r></w:p></w:tc></w:tr>
    </w:tbl>
  </w:body>
</w:document>`
	writeZip(t, path, map[string]string{"word/document.xml": documentXML})

	res, err := File(path)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !strings.Contains(res.Text, "# Titre du document") {
		t.Errorf("heading not rendered: %q", res.Text)
	}
	if !strings.Contains(res.Text, "Un paragraphe normal.") {
		t.Errorf("paragraph missing: %q", res.Text)
	}
	if !strings.Contains(res.Text, "| A | B |") {
		t.Errorf("table not rendered as markdown: %q", res.Text)
	}
}

// TestReadZipMemberRefusesOversizedEntry guards against a real DoS
// vector: a .docx/.xlsx/.pptx (all three go through this one shared
// helper) whose on-disk size is small but whose XML member
// decompresses to something enormous. Before this bound existed,
// uploading a file like that as a chat attachment would make the
// daemon buffer the entire decompressed member in memory with no cap
// at all -- a classic zip bomb. Exercised via readZipMemberLimited
// with a small limit rather than the real 200MB maxZipMemberBytes:
// the bound-checking logic is identical either way, and a multi-
// hundred-MB fixture would be slow to generate/compress/decompress in
// a unit test even though it compresses down to almost nothing on
// disk.
func TestReadZipMemberRefusesOversizedEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bomb.docx")
	const limit = 1024
	oversized := strings.Repeat("A", limit+1)
	writeZip(t, path, map[string]string{"word/document.xml": oversized})

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader: %v", err)
	}
	defer r.Close()

	if _, err := readZipMemberLimited(r, "word/document.xml", limit); err == nil {
		t.Error("expected a member decompressing past the limit to be refused, not silently read into memory")
	}
}

func TestReadZipMemberAllowsEntryAtTheLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.docx")
	const limit = 1024
	exact := strings.Repeat("A", limit)
	writeZip(t, path, map[string]string{"word/document.xml": exact})

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader: %v", err)
	}
	defer r.Close()

	data, err := readZipMemberLimited(r, "word/document.xml", limit)
	if err != nil {
		t.Fatalf("readZipMemberLimited: %v", err)
	}
	if len(data) != limit {
		t.Errorf("got %d bytes, want exactly %d", len(data), limit)
	}
}

func TestConvertXLSX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.xlsx")
	workbookXML := `<?xml version="1.0"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets><sheet name="Feuille1" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/></sheets>
</workbook>`
	sstXML := `<?xml version="1.0"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="1" uniqueCount="1">
  <si><t>Nom</t></si>
</sst>`
	sheetXML := `<?xml version="1.0"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1"><v>42</v></c></row>
  </sheetData>
</worksheet>`
	writeZip(t, path, map[string]string{
		"xl/workbook.xml":          workbookXML,
		"xl/sharedStrings.xml":     sstXML,
		"xl/worksheets/sheet1.xml": sheetXML,
	})

	res, err := File(path)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !strings.Contains(res.Text, "Feuille1") {
		t.Errorf("sheet name missing: %q", res.Text)
	}
	if !strings.Contains(res.Text, "| Nom | 42 |") {
		t.Errorf("row content not rendered: %q", res.Text)
	}
}

func TestConvertPPTX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.pptx")
	slideXML := `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Bienvenue chez Runyx</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`
	writeZip(t, path, map[string]string{"ppt/slides/slide1.xml": slideXML})

	res, err := File(path)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !strings.Contains(res.Text, "Bienvenue chez Runyx") {
		t.Errorf("slide text missing: %q", res.Text)
	}
	if !strings.Contains(res.Text, "## Slide 1") {
		t.Errorf("slide heading missing: %q", res.Text)
	}
}

func TestChunkShortTextIsOneChunk(t *testing.T) {
	chunks := Chunk("texte court", 2000, 100)
	if len(chunks) != 1 || chunks[0] != "texte court" {
		t.Errorf("got %v", chunks)
	}
}

func TestChunkSplitsLongText(t *testing.T) {
	para := strings.Repeat("mot ", 100) // ~500 chars
	text := strings.Join([]string{para, para, para, para, para}, "\n\n")
	chunks := Chunk(text, 600, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c) > 700 { // budget + some paragraph overflow tolerance
			t.Errorf("chunk too large: %d chars", len(c))
		}
	}
}

func TestUnsupportedExtensionErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "video.mp4")
	os.WriteFile(path, []byte("not really a video"), 0o644)
	if _, err := File(path); err == nil {
		t.Error("expected error for unsupported extension via File()")
	}
}
