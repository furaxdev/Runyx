// Package convert turns arbitrary attached files into model-ready text,
// so that any model — even one with no native file support — can receive
// any file. Implements FEATURES.md item 17 (universal attachments).
//
// Structure is preserved where it matters: page/slide boundaries and
// headings become Markdown headings, spreadsheet rows become Markdown
// tables. Content too large for a model's context is chunked (see
// Chunk) for retrieval instead of being silently truncated — the
// retrieval side of that lives in internal/rag (item 16).
package convert

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Result is a converted attachment: the text handed to the model, plus
// metadata shown to the user so they can see exactly what was sent (the
// "pastille" from item 17).
type Result struct {
	FileName  string
	Kind      string // "text", "pdf", "docx", "xlsx", "pptx", "archive", "audio", "video", "image", "code"
	Text      string
	Truncated bool
	Warnings  []string
}

// textExtensions are read as-is (UTF-8 assumed); this covers plain text
// and virtually all source code files.
var textExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".csv": true, ".tsv": true,
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
	".html": true, ".htm": true, ".log": true,
	".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".java": true, ".c": true, ".h": true, ".cpp": true, ".hpp": true, ".cs": true,
	".rs": true, ".rb": true, ".php": true, ".sh": true, ".sql": true, ".kt": true,
	".swift": true, ".lua": true, ".pl": true, ".r": true, ".scala": true,
}

// File converts a single file on disk into model-ready text. Audio/video
// handling needs an external transcriber (see ConvertAudio/ConvertVideo
// in internal/voice integration); File handles everything that's pure
// Go: text/code, PDF, DOCX/XLSX/PPTX, and archive listings.
func File(path string) (Result, error) {
	ext := strings.ToLower(filepath.Ext(path))
	name := filepath.Base(path)

	switch {
	case textExtensions[ext]:
		return convertText(path, name)
	case ext == ".pdf":
		return convertPDF(path, name)
	case ext == ".docx":
		return convertDOCX(path, name)
	case ext == ".xlsx":
		return convertXLSX(path, name)
	case ext == ".pptx":
		return convertPPTX(path, name)
	case ext == ".zip":
		return convertArchive(path, name)
	case audioExtensions[ext]:
		return Result{}, errNeedsTranscription{Kind: "audio"}
	case videoExtensions[ext]:
		return Result{}, errNeedsTranscription{Kind: "video"}
	default:
		return Result{}, fmt.Errorf("unsupported file type %q (no converter for %s)", ext, name)
	}
}

var audioExtensions = map[string]bool{
	".mp3": true, ".wav": true, ".m4a": true, ".flac": true, ".ogg": true, ".opus": true,
}

var videoExtensions = map[string]bool{
	".mp4": true, ".mov": true, ".mkv": true, ".webm": true, ".avi": true,
}

// errNeedsTranscription signals that converting this file needs the
// voice pipeline (internal/voice), which File() doesn't depend on to
// keep convert dependency-free; callers with a voice.Transcriber should
// catch this and call AudioToText/VideoToText instead.
type errNeedsTranscription struct{ Kind string }

func (e errNeedsTranscription) Error() string {
	return fmt.Sprintf("%s files need local transcription (internal/voice); call convert.AudioToText or convert.VideoToText", e.Kind)
}

// NeedsTranscription reports whether err indicates a file that File()
// recognized but can't convert without a voice.Transcriber.
func NeedsTranscription(err error) bool {
	_, ok := err.(errNeedsTranscription)
	return ok
}

func convertText(path, name string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", name, err)
	}
	kind := "text"
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".txt" && ext != ".md" && ext != ".markdown" && ext != ".csv" && ext != ".tsv" && ext != ".log" {
		kind = "code"
	}
	return Result{FileName: name, Kind: kind, Text: string(data)}, nil
}

// convertArchive lists a zip's contents and inlines any text/code member
// files (keeping the archive's folder structure as Markdown headings),
// skipping binaries.
func convertArchive(path, name string) (Result, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return Result{}, fmt.Errorf("open archive %s: %w", name, err)
	}
	defer r.Close()

	var b strings.Builder
	fmt.Fprintf(&b, "# Archive: %s\n\n", name)

	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	fmt.Fprintf(&b, "## Contents (%d files)\n\n", len(names))
	for _, n := range names {
		fmt.Fprintf(&b, "- %s\n", n)
	}

	var warnings []string
	const maxInlinedPerFile = 50_000 // characters
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if !textExtensions[ext] {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("could not read %s: %v", f.Name, err))
			continue
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxInlinedPerFile+1))
		rc.Close()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("could not read %s: %v", f.Name, err))
			continue
		}
		truncatedNote := ""
		if len(content) > maxInlinedPerFile {
			content = content[:maxInlinedPerFile]
			truncatedNote = " (truncated)"
		}
		fmt.Fprintf(&b, "\n### %s%s\n\n```\n%s\n```\n", f.Name, truncatedNote, string(content))
	}

	return Result{FileName: name, Kind: "archive", Text: b.String(), Warnings: warnings}, nil
}
