// Block-level markdown parsing: headings, paragraphs, lists (incl.
// nested via indentation depth, and task-list checkboxes), blockquotes,
// fenced code blocks, aligned tables, and thematic breaks.
//
// Streaming safety: an unclosed code fence (very common mid-stream — a
// reply that's still inside ```...``` when more tokens haven't arrived
// yet) is emitted as a codeBlock node with `closed: false` containing
// everything parsed so far, rather than being swallowed into a
// paragraph or causing the parser to wait/throw. The caller re-parses
// the full accumulated text on every update (cheap for chat-message-
// sized text — this is "incremental" in the sense of "safe and cheap to
// re-run continuously", not true incremental AST diffing; see
// docs/DECISIONS.md).
//
// This is not a full CommonMark implementation: list items are treated
// as single-paragraph (no multi-paragraph list items, no lazy
// continuation lines), and setext headings (text\n===) aren't
// recognized — both documented simplifications.
import { parseInline } from "./inline.js";
import type { BlockNode, ListItem } from "./types.js";

interface Line {
  text: string;
  start: number; // offset of this line's first character in the source
  end: number; // offset just past this line's content (before the \n)
}

function splitLines(source: string): Line[] {
  const lines: Line[] = [];
  let offset = 0;
  for (const raw of source.split("\n")) {
    lines.push({ text: raw, start: offset, end: offset + raw.length });
    offset += raw.length + 1;
  }
  return lines;
}

const HEADING_RE = /^(#{1,6})\s+(.*)$/;
const RULE_RE = /^ {0,3}([-*_])(?:\s*\1){2,}\s*$/;
const FENCE_RE = /^ {0,3}```\s*(\S*)\s*$/;
const LIST_RE = /^(\s*)([-*+]|\d+[.)])\s+(.*)$/;
const TASK_RE = /^\[([ xX])\]\s+(.*)$/;
const QUOTE_RE = /^ {0,3}>\s?(.*)$/;
const TABLE_SEP_RE = /^\s*\|?(?:\s*:?-+:?\s*\|)+\s*:?-+:?\s*\|?\s*$/;

export function parseBlocks(source: string): BlockNode[] {
  const lines = splitLines(source);
  const blocks: BlockNode[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i]!;

    if (line.text.trim() === "") {
      blocks.push({ kind: "blank", start: line.start, end: line.end });
      i++;
      continue;
    }

    const fence = FENCE_RE.exec(line.text);
    if (fence) {
      const lang = fence[1] ?? "";
      const codeStart = line.start;
      const bodyLines: string[] = [];
      let j = i + 1;
      let closed = false;
      let end = line.end;
      while (j < lines.length) {
        if (FENCE_RE.test(lines[j]!.text) || /^ {0,3}```\s*$/.test(lines[j]!.text)) {
          closed = true;
          end = lines[j]!.end;
          j++;
          break;
        }
        bodyLines.push(lines[j]!.text);
        end = lines[j]!.end;
        j++;
      }
      blocks.push({ kind: "codeBlock", lang, code: bodyLines.join("\n"), closed, start: codeStart, end });
      i = j;
      continue;
    }

    if (RULE_RE.test(line.text)) {
      blocks.push({ kind: "rule", start: line.start, end: line.end });
      i++;
      continue;
    }

    const heading = HEADING_RE.exec(line.text);
    if (heading) {
      const level = heading[1]!.length as 1 | 2 | 3 | 4 | 5 | 6;
      const textStart = line.start + heading[1]!.length + 1;
      blocks.push({
        kind: "heading",
        level,
        children: parseInline(heading[2]!, textStart),
        start: line.start,
        end: line.end,
      });
      i++;
      continue;
    }

    if (QUOTE_RE.test(line.text)) {
      const quoteLines: string[] = [];
      const start = line.start;
      let end = line.end;
      let j = i;
      while (j < lines.length && QUOTE_RE.test(lines[j]!.text)) {
        quoteLines.push(QUOTE_RE.exec(lines[j]!.text)![1]!);
        end = lines[j]!.end;
        j++;
      }
      blocks.push({ kind: "blockquote", children: parseBlocks(quoteLines.join("\n")), start, end });
      i = j;
      continue;
    }

    if (isTableStart(lines, i)) {
      const { node, nextIndex } = parseTable(lines, i);
      blocks.push(node);
      i = nextIndex;
      continue;
    }

    if (LIST_RE.test(line.text)) {
      const { node, nextIndex } = parseList(lines, i);
      blocks.push(node);
      i = nextIndex;
      continue;
    }

    // Paragraph: consume consecutive plain lines.
    const start = line.start;
    let end = line.end;
    const textLines = [line.text];
    let j = i + 1;
    while (j < lines.length && lines[j]!.text.trim() !== "" && !isBlockStart(lines[j]!.text)) {
      textLines.push(lines[j]!.text);
      end = lines[j]!.end;
      j++;
    }
    blocks.push({ kind: "paragraph", children: parseInline(textLines.join("\n"), start), start, end });
    i = j;
  }

  return blocks;
}

function isBlockStart(text: string): boolean {
  return HEADING_RE.test(text) || RULE_RE.test(text) || FENCE_RE.test(text) || QUOTE_RE.test(text) || LIST_RE.test(text);
}

function isTableStart(lines: Line[], i: number): boolean {
  const line = lines[i]!;
  if (!line.text.includes("|")) return false;
  const next = lines[i + 1];
  return !!next && TABLE_SEP_RE.test(next.text);
}

function splitRow(text: string): string[] {
  const trimmed = text.trim().replace(/^\|/, "").replace(/\|$/, "");
  return trimmed.split("|").map((c) => c.trim());
}

function parseTable(lines: Line[], i: number): { node: BlockNode; nextIndex: number } {
  const headerLine = lines[i]!;
  const sepLine = lines[i + 1]!;
  const header = splitRow(headerLine.text).map((c) => parseInline(c));
  const align = splitRow(sepLine.text).map((cell): "left" | "center" | "right" => {
    const l = cell.startsWith(":");
    const r = cell.endsWith(":");
    if (l && r) return "center";
    if (r) return "right";
    return "left";
  });
  const rows: ReturnType<typeof parseInline>[][] = [];
  let j = i + 2;
  let end = sepLine.end;
  while (j < lines.length && lines[j]!.text.includes("|") && lines[j]!.text.trim() !== "") {
    rows.push(splitRow(lines[j]!.text).map((c) => parseInline(c)));
    end = lines[j]!.end;
    j++;
  }
  return { node: { kind: "table", header, align, rows, start: headerLine.start, end }, nextIndex: j };
}

function parseList(lines: Line[], i: number): { node: BlockNode; nextIndex: number } {
  const first = LIST_RE.exec(lines[i]!.text)!;
  const ordered = /\d/.test(first[2]!);
  const items: ListItem[] = [];
  const start = lines[i]!.start;
  let end = lines[i]!.end;
  let j = i;
  while (j < lines.length) {
    const m = LIST_RE.exec(lines[j]!.text);
    if (!m) break;
    const indent = m[1]!.length;
    const depth = Math.floor(indent / 2);
    let itemText = m[3]!;
    let checked: boolean | undefined;
    const task = TASK_RE.exec(itemText);
    if (task) {
      checked = task[1]!.toLowerCase() === "x";
      itemText = task[2]!;
    }
    const itemStart = lines[j]!.start;
    let itemEnd = lines[j]!.end;
    end = itemEnd;
    const textOffset = itemStart + (lines[j]!.text.length - m[3]!.length) + (task ? task[0]!.length - task[2]!.length : 0);
    items.push({
      start: itemStart,
      end: itemEnd,
      depth,
      checked,
      children: [{ kind: "paragraph", children: parseInline(itemText, textOffset), start: itemStart, end: itemEnd }],
    });
    j++;
  }
  return { node: { kind: "list", ordered, items, start, end }, nextIndex: j };
}
