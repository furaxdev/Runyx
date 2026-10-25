// Inline markdown parsing: bold/italic/bold-italic/strike/code/links.
// Streaming-safe by construction: each syntax form is only recognized
// once BOTH its opening and closing markers are present in the text;
// an unterminated marker (the common case mid-stream, e.g. "**bold"
// with no closing "**" yet) is simply left as literal text and
// re-evaluated as more text arrives on the next render — there is no
// "unterminated marker" error state to get stuck in.
import type { InlineNode } from "./types.js";

interface Match {
  index: number;
  length: number;
  build: (baseOffset: number) => InlineNode;
}

function findCode(text: string, from: number): Match | null {
  const open = text.indexOf("`", from);
  if (open === -1) return null;
  const close = text.indexOf("`", open + 1);
  if (close === -1) return null;
  const inner = text.slice(open + 1, close);
  return {
    index: open,
    length: close + 1 - open,
    build: (base) => ({ kind: "code", text: inner, start: base + open, end: base + close + 1 }),
  };
}

function findLink(text: string, from: number): Match | null {
  const re = /\[([^\]\n]*)\]\(([^)\s]+)\)/g;
  re.lastIndex = from;
  const m = re.exec(text);
  if (!m || m.index < from) return null;
  const linkText = m[1]!;
  const url = m[2]!;
  const len = m[0]!.length;
  return {
    index: m.index,
    length: len,
    build: (base) => ({ kind: "link", text: linkText, url, start: base + m.index, end: base + m.index + len }),
  };
}

function findWrapped(text: string, from: number, marker: string, kind: "bold" | "italic" | "boldItalic" | "strike"): Match | null {
  let searchFrom = from;
  while (true) {
    const open = text.indexOf(marker, searchFrom);
    if (open === -1) return null;
    // The char right after the opener must not be whitespace (left-flanking).
    const afterOpen = text[open + marker.length];
    if (afterOpen === undefined || /\s/.test(afterOpen)) {
      searchFrom = open + marker.length;
      continue;
    }
    const innerStart = open + marker.length;
    const close = text.indexOf(marker, innerStart);
    if (close === -1) return null;
    if (close === innerStart) {
      // Empty content (e.g. the second "**" of an unterminated "**word"
      // being misread as italic-of-nothing from the adjacent "*"+"*") —
      // not a valid emphasis span; keep looking further in the text.
      searchFrom = open + 1;
      continue;
    }
    // The char right before the closer must not be whitespace (right-flanking).
    const beforeClose = text[close - 1];
    if (beforeClose !== undefined && /\s/.test(beforeClose)) {
      searchFrom = open + marker.length;
      continue;
    }
    const inner = text.slice(innerStart, close);
    const len = close + marker.length - open;
    return {
      index: open,
      length: len,
      build: (base) => ({
        kind,
        children: parseInline(inner, base + innerStart),
        start: base + open,
        end: base + close + marker.length,
      }),
    };
  }
}

/** Parses inline markdown in `text`, offsetting every node's [start,end) by `baseOffset` (the text's position in the full source). */
export function parseInline(text: string, baseOffset = 0): InlineNode[] {
  const nodes: InlineNode[] = [];
  let cursor = 0;
  let plainStart = 0;

  const flushPlain = (upTo: number) => {
    if (upTo > plainStart) {
      nodes.push({ kind: "text", text: text.slice(plainStart, upTo), start: baseOffset + plainStart, end: baseOffset + upTo });
    }
  };

  while (cursor < text.length) {
    const candidates: Match[] = [];
    const code = findCode(text, cursor);
    if (code) candidates.push(code);
    const link = findLink(text, cursor);
    if (link) candidates.push(link);
    const boldItalic = findWrapped(text, cursor, "***", "boldItalic");
    if (boldItalic) candidates.push(boldItalic);
    const bold = findWrapped(text, cursor, "**", "bold");
    if (bold) candidates.push(bold);
    const strike = findWrapped(text, cursor, "~~", "strike");
    if (strike) candidates.push(strike);
    const italicStar = findWrapped(text, cursor, "*", "italic");
    if (italicStar) candidates.push(italicStar);
    const italicUnderscore = findWrapped(text, cursor, "_", "italic");
    if (italicUnderscore) candidates.push(italicUnderscore);

    if (candidates.length === 0) break;
    candidates.sort((a, b) => a.index - b.index || b.length - a.length);
    const winner = candidates[0]!;

    flushPlain(winner.index);
    nodes.push(winner.build(baseOffset));
    cursor = winner.index + winner.length;
    plainStart = cursor;
  }
  flushPlain(text.length);
  return nodes;
}

/** Flattens an inline tree back into plain text (used by "copy as text"/selection extraction). */
export function inlineToPlainText(nodes: InlineNode[]): string {
  let out = "";
  for (const n of nodes) {
    if (n.kind === "text" || n.kind === "code") out += n.text;
    else if (n.kind === "link") out += n.text;
    else out += inlineToPlainText(n.children);
  }
  return out;
}
