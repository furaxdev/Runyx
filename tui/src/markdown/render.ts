// Turns a parsed markdown AST into a flat list of "rendered lines" —
// the layout step, independent of any Screen so it's cheap to call on
// every keystroke/streamed token and cheap to unit-test. Each line
// carries the exact [start,end) source-text range it came from (for
// content-anchored selection) and a plain-text form (for clean copy:
// no box-drawing borders, no list bullets/line numbers baked into the
// copyable text beyond what the user actually wrote).
import { highlightLine, type Token } from "./highlight.js";
import { graphemes, stringWidth } from "../term/width.js";
import { wrapText } from "../widgets/text.js";
import type { BlockNode, InlineNode, SourceSpan } from "./types.js";
import { inlineToPlainText } from "./inline.js";

export interface StyledSpan {
  text: string;
  bold?: boolean;
  italic?: boolean;
  strike?: boolean;
  underline?: boolean;
  dim?: boolean;
  colorToken?: "accent" | "muted" | "danger" | "success" | "warning" | "keyword" | "string" | "comment" | "number" | "type";
  linkUrl?: string;
}

export interface RenderedLine {
  spans: StyledSpan[];
  plainText: string;
  source: SourceSpan | null;
  codeBlockId?: string;
  codeBlockLang?: string;
  isCodeBlockHeader?: boolean;
  isCodeBlockFooter?: boolean;
  /**
   * True for every wrapped visual line after the first one produced
   * from a single logical paragraph/heading/list-item line — i.e. a
   * soft wrap the terminal width forced, not a line break the user (or
   * the model) actually wrote. Selection/copy joins these with a
   * space; real line breaks (code block lines, separate list items,
   * table rows, paragraph boundaries) join with "\n". Never set on
   * code block lines, which are never soft-wrapped.
   */
  wrapContinuation?: boolean;
}

export interface RenderOptions {
  width: number;
  italicSupported: boolean;
}

let codeBlockCounter = 0;

export function renderMarkdown(blocks: BlockNode[], opts: RenderOptions): RenderedLine[] {
  const out: RenderedLine[] = [];
  renderBlocks(blocks, opts, 0, out);
  return out;
}

function renderBlocks(blocks: BlockNode[], opts: RenderOptions, indent: number, out: RenderedLine[]): void {
  for (const b of blocks) renderBlock(b, opts, indent, out);
}

function renderBlock(b: BlockNode, opts: RenderOptions, indent: number, out: RenderedLine[]): void {
  const width = Math.max(1, opts.width - indent);
  switch (b.kind) {
    case "blank":
      out.push({ spans: [{ text: "" }], plainText: "", source: b });
      return;
    case "heading": {
      const plain = inlineToPlainText(b.children);
      const prefix = "#".repeat(b.level) + " ";
      wrapText(plain, width - prefix.length).forEach((line, i) => {
        out.push({
          spans: [{ text: indentStr(indent) + prefix + line, bold: true, colorToken: "accent" }],
          plainText: line,
          source: b,
          wrapContinuation: i > 0,
        });
      });
      return;
    }
    case "paragraph": {
      const spanLines = wrapInline(b.children, width, opts.italicSupported);
      spanLines.forEach((line, i) => out.push({ spans: prefixSpans(indentStr(indent), line.spans), plainText: line.plainText, source: b, wrapContinuation: i > 0 }));
      return;
    }
    case "rule":
      out.push({ spans: [{ text: indentStr(indent) + "─".repeat(Math.max(1, width)), dim: true }], plainText: "---", source: b });
      return;
    case "list": {
      for (const item of b.items) {
        const bulletIndent = indent + item.depth * 2;
        const bullet = item.checked === undefined ? (b.ordered ? "1." : "•") : item.checked ? "[x]" : "[ ]";
        const para = item.children[0];
        const inline = para && para.kind === "paragraph" ? para.children : [];
        const lines = wrapInline(inline, Math.max(1, opts.width - bulletIndent - bullet.length - 1), opts.italicSupported);
        lines.forEach((line, i) => {
          const prefix = i === 0 ? `${indentStr(bulletIndent)}${bullet} ` : indentStr(bulletIndent + bullet.length + 1);
          out.push({
            spans: prefixSpans(prefix, line.spans, item.checked === true ? { strike: true, dim: true } : {}),
            plainText: line.plainText,
            source: item,
            wrapContinuation: i > 0,
          });
        });
      }
      return;
    }
    case "blockquote": {
      const innerOut: RenderedLine[] = [];
      renderBlocks(b.children, { ...opts, width: opts.width - indent - 2 }, 0, innerOut);
      for (const line of innerOut) {
        out.push({ spans: prefixSpans(indentStr(indent) + "│ ", line.spans, { dim: true, colorToken: "muted" }), plainText: line.plainText, source: line.source });
      }
      return;
    }
    case "codeBlock": {
      const id = `code-${codeBlockCounter++}`;
      out.push({ spans: [{ text: `${indentStr(indent)}${b.lang || "text"}`, dim: true }], plainText: "", source: b, codeBlockId: id, codeBlockLang: b.lang, isCodeBlockHeader: true });
      const codeLines = b.code === "" ? [] : b.code.split("\n");
      for (const line of codeLines) {
        const tokens = highlightLine(line, b.lang);
        out.push({ spans: tokens.map(tokenToSpan), plainText: line, source: b, codeBlockId: id, codeBlockLang: b.lang });
      }
      if (b.closed) {
        out.push({ spans: [{ text: indentStr(indent), dim: true }], plainText: "", source: b, codeBlockId: id, isCodeBlockFooter: true });
      }
      return;
    }
    case "table": {
      const colWidths = b.header.map((cell, i) => {
        const headerW = stringWidth(inlineToPlainText(cell));
        const bodyW = Math.max(0, ...b.rows.map((row) => stringWidth(inlineToPlainText(row[i] ?? []))));
        return Math.max(3, headerW, bodyW);
      });
      out.push({ spans: [tableRowSpan(b.header, colWidths, b.align, true)], plainText: inlineToPlainText(b.header.flat()), source: b });
      out.push({ spans: [{ text: colWidths.map((w) => "─".repeat(w + 2)).join("┼"), dim: true }], plainText: "", source: b });
      for (const row of b.rows) {
        out.push({ spans: [tableRowSpan(row, colWidths, b.align, false)], plainText: inlineToPlainText(row.flat()), source: b });
      }
      return;
    }
  }
}

function tableRowSpan(cells: InlineNode[][], widths: number[], align: ("left" | "center" | "right")[], header: boolean): StyledSpan {
  const parts = cells.map((cell, i) => {
    const text = inlineToPlainText(cell);
    const w = widths[i] ?? 3;
    return padAlign(text, w, align[i] ?? "left");
  });
  return { text: parts.join(" │ "), bold: header };
}

function padAlign(text: string, width: number, align: "left" | "center" | "right"): string {
  const w = stringWidth(text);
  const gap = Math.max(0, width - w);
  if (align === "right") return " ".repeat(gap) + text;
  if (align === "center") return " ".repeat(Math.floor(gap / 2)) + text + " ".repeat(Math.ceil(gap / 2));
  return text + " ".repeat(gap);
}

type ColorToken = NonNullable<StyledSpan["colorToken"]>;
function tokenToSpan(t: Token): StyledSpan {
  if (t.kind === "plain") return { text: t.text };
  return { text: t.text, colorToken: t.kind as ColorToken };
}

function indentStr(n: number): string {
  return " ".repeat(Math.max(0, n));
}

function prefixSpans(prefix: string, spans: StyledSpan[], extra: Partial<StyledSpan> = {}): StyledSpan[] {
  if (spans.length === 0) return [{ text: prefix, ...extra }];
  return [{ text: prefix }, ...spans.map((s) => ({ ...s, ...extra }))];
}

interface WrappedInlineLine {
  spans: StyledSpan[];
  plainText: string;
}

/** Flattens inline nodes to styled spans, then wraps to `width`, splitting spans across line breaks as needed. */
function wrapInline(nodes: InlineNode[], width: number, italicSupported: boolean): WrappedInlineLine[] {
  const flatSpans = flattenInline(nodes, italicSupported);
  const plain = flatSpans.map((s) => s.text).join("");
  const wrappedPlainLines = wrapText(plain, Math.max(1, width));

  // Re-split flatSpans to match each wrapped plain-text line's length.
  const lines: WrappedInlineLine[] = [];
  let spanIdx = 0;
  let spanOffset = 0;
  for (const plainLine of wrappedPlainLines) {
    let remaining = Array.from(plainLine).length;
    // Skip whitespace consumed by wrapText between words that isn't in plainLine.
    const lineSpans: StyledSpan[] = [];
    while (remaining > 0 && spanIdx < flatSpans.length) {
      const span = flatSpans[spanIdx]!;
      const chars = graphemes(span.text);
      const available = chars.length - spanOffset;
      const take = Math.min(available, remaining);
      const text = chars
        .slice(spanOffset, spanOffset + take)
        .map((g) => g.text)
        .join("");
      if (text.length > 0) lineSpans.push({ ...span, text });
      spanOffset += take;
      remaining -= take;
      if (spanOffset >= chars.length) {
        spanIdx++;
        spanOffset = 0;
      }
    }
    lines.push({ spans: lineSpans, plainText: plainLine });
    // Account for the whitespace between words that wrapText trimmed.
    while (spanIdx < flatSpans.length && /^\s+$/.test(flatSpans[spanIdx]!.text.slice(spanOffset, spanOffset + 1))) {
      spanOffset++;
      if (spanOffset >= flatSpans[spanIdx]!.text.length) {
        spanIdx++;
        spanOffset = 0;
      } else {
        break;
      }
    }
  }
  return lines;
}

function flattenInline(nodes: InlineNode[], italicSupported: boolean, inherited: Partial<StyledSpan> = {}): StyledSpan[] {
  const out: StyledSpan[] = [];
  for (const n of nodes) {
    if (n.kind === "text") {
      out.push({ text: n.text, ...inherited });
    } else if (n.kind === "code") {
      out.push({ text: n.text, colorToken: "string", ...inherited });
    } else if (n.kind === "link") {
      out.push({ text: n.text, linkUrl: n.url, underline: true, colorToken: "accent", ...inherited });
    } else if (n.kind === "bold") {
      out.push(...flattenInline(n.children, italicSupported, { ...inherited, bold: true }));
    } else if (n.kind === "italic") {
      out.push(...flattenInline(n.children, italicSupported, italicSupported ? { ...inherited, italic: true } : { ...inherited, underline: true }));
    } else if (n.kind === "boldItalic") {
      out.push(...flattenInline(n.children, italicSupported, italicSupported ? { ...inherited, bold: true, italic: true } : { ...inherited, bold: true, underline: true }));
    } else if (n.kind === "strike") {
      out.push(...flattenInline(n.children, italicSupported, { ...inherited, strike: true }));
    }
  }
  return out;
}
