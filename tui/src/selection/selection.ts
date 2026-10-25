// Mouse-driven text selection over a document of RenderedLine[]. Fully
// content-anchored: a selection is a [start, end) pair of offsets into
// a *synthetic flattened text* built fresh from the current
// RenderedLine[] every render — never screen row/column — so it stays
// correct across scrolling (which only changes which lines are
// visible, not their offsets) and streaming (new tokens only ever
// append lines/extend the last line, so every earlier offset stays
// valid; see docs/DECISIONS.md for why this is "safe to rebuild every
// frame" rather than a persistent incremental structure).
import type { RenderedLine } from "../markdown/render.js";

export interface FlatLine {
  lineIndex: number;
  start: number;
  end: number;
  line: RenderedLine;
}

export interface SelectionRange {
  start: number;
  end: number;
}

/** Rebuilds the flat offset index from the current rendered lines. Cheap: O(total visible+scrolled text), called once per frame when a selection is active. */
export function buildFlatIndex(lines: RenderedLine[]): FlatLine[] {
  let offset = 0;
  const out: FlatLine[] = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]!;
    const len = Array.from(line.plainText).length;
    out.push({ lineIndex: i, start: offset, end: offset + len, line });
    offset += len + 1; // +1: a synthetic separator so adjacent lines never share an offset
  }
  return out;
}

export function totalLength(flat: FlatLine[]): number {
  const last = flat[flat.length - 1];
  return last ? last.end : 0;
}

export function lineAtOffset(flat: FlatLine[], offset: number): FlatLine | undefined {
  // Linear scan: simple and fast enough (one pass over already-built
  // flat index); binary search would need a monotonic invariant that
  // holds here too, but isn't worth the complexity at chat-transcript
  // sizes.
  for (const f of flat) {
    if (offset >= f.start && offset <= f.end) return f;
  }
  return flat[flat.length - 1];
}

/** Converts a (lineIndex, column-in-that-line) position to a flat offset. */
export function positionToOffset(flat: FlatLine[], lineIndex: number, column: number): number {
  const f = flat.find((x) => x.lineIndex === lineIndex);
  if (!f) return totalLength(flat);
  const lineLen = f.end - f.start;
  return f.start + Math.max(0, Math.min(lineLen, column));
}

const WORD_CHAR = /[\p{L}\p{N}_]/u;

/** Expands a click position to the enclosing word's [start,end) range (double-click). */
export function wordRangeAt(flat: FlatLine[], offset: number): SelectionRange {
  const f = lineAtOffset(flat, offset);
  if (!f) return { start: offset, end: offset };
  const chars = Array.from(f.line.plainText);
  let col = offset - f.start;
  if (col >= chars.length) return { start: offset, end: offset };
  if (!WORD_CHAR.test(chars[col] ?? "")) return { start: offset, end: offset + 1 };
  let left = col;
  while (left > 0 && WORD_CHAR.test(chars[left - 1] ?? "")) left--;
  let right = col;
  while (right < chars.length && WORD_CHAR.test(chars[right] ?? "")) right++;
  return { start: f.start + left, end: f.start + right };
}

/** The whole rendered line containing offset (triple-click). */
export function lineRangeAt(flat: FlatLine[], offset: number): SelectionRange {
  const f = lineAtOffset(flat, offset);
  if (!f) return { start: offset, end: offset };
  return { start: f.start, end: f.end };
}

export function normalizeRange(a: number, b: number): SelectionRange {
  return a <= b ? { start: a, end: b } : { start: b, end: a };
}

/**
 * Extracts clean, copy-ready text for [start,end): soft-wrapped visual
 * lines (RenderedLine.wrapContinuation) are rejoined with a space (the
 * line break was the terminal's doing, not the author's); every other
 * line boundary (a real line break — code lines, list items, table
 * rows, paragraph/heading boundaries) joins with "\n". Trailing
 * whitespace is stripped from each piece. No box-drawing borders, list
 * bullets, or other UI chrome is ever included — only `plainText`,
 * which never contains them.
 */
export function extractText(flat: FlatLine[], range: SelectionRange): string {
  const pieces: { text: string; continuation: boolean }[] = [];
  for (const f of flat) {
    const lineLen = f.end - f.start;
    const sliceStart = Math.max(0, range.start - f.start);
    const sliceEnd = Math.min(lineLen, range.end - f.start);
    if (sliceStart >= sliceEnd) {
      if (lineLen === 0 && range.start <= f.start && range.end >= f.end) {
        pieces.push({ text: "", continuation: !!f.line.wrapContinuation });
      }
      continue;
    }
    const chars = Array.from(f.line.plainText);
    const text = chars
      .slice(sliceStart, sliceEnd)
      .join("")
      .replace(/\s+$/, "");
    pieces.push({ text, continuation: !!f.line.wrapContinuation });
  }
  let out = "";
  pieces.forEach((p, i) => {
    if (i === 0) {
      out = p.text;
      return;
    }
    out += (p.continuation ? " " : "\n") + p.text;
  });
  return out;
}

/**
 * "Copy as Markdown": extracts the *original markdown source* covering
 * [start,end) (via each line's SourceSpan) instead of the rendered
 * plain text, so code fences/emphasis markers/link syntax are
 * preserved verbatim rather than flattened.
 */
export function extractMarkdownSource(flat: FlatLine[], range: SelectionRange, source: string): string {
  let min = Infinity;
  let max = -Infinity;
  for (const f of flat) {
    if (f.end < range.start || f.start > range.end) continue;
    if (!f.line.source) continue;
    min = Math.min(min, f.line.source.start);
    max = Math.max(max, f.line.source.end);
  }
  if (!isFinite(min)) return extractText(flat, range);
  return source.slice(min, max);
}

/** All lines belonging to one code block (by its stable id), real-newline-joined (never space-joined — code lines are never wrap continuations). Used for the per-code-block Copy button and "copy Nth code block" shortcut. */
export function extractCodeBlock(lines: RenderedLine[], codeBlockId: string): string {
  return lines
    .filter((l) => l.codeBlockId === codeBlockId && !l.isCodeBlockHeader && !l.isCodeBlockFooter)
    .map((l) => l.plainText)
    .join("\n");
}

export function selectAllRange(flat: FlatLine[]): SelectionRange {
  return { start: 0, end: totalLength(flat) };
}
