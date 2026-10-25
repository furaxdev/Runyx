import { describe, expect, test } from "bun:test";
import { parseBlocks } from "../src/markdown/block.js";
import { renderMarkdown, type RenderedLine } from "../src/markdown/render.js";
import {
  buildFlatIndex,
  extractCodeBlock,
  extractMarkdownSource,
  extractText,
  lineRangeAt,
  normalizeRange,
  selectAllRange,
  totalLength,
  wordRangeAt,
} from "../src/selection/selection.js";

function render(src: string, width = 80) {
  return renderMarkdown(parseBlocks(src), { width, italicSupported: true });
}

function line(plainText: string, extra: Partial<RenderedLine> = {}): RenderedLine {
  return { spans: [{ text: plainText }], plainText, source: null, ...extra };
}

describe("word selection (double-click)", () => {
  test("selects the whole word under a click inside it", () => {
    const lines = [line("the quick brown fox")];
    const flat = buildFlatIndex(lines);
    const range = wordRangeAt(flat, 4 + 2); // inside "quick"
    expect(extractText(flat, range)).toBe("quick");
  });

  test("clicking on punctuation selects just that character", () => {
    const lines = [line("hello, world")];
    const flat = buildFlatIndex(lines);
    const range = wordRangeAt(flat, 5); // the comma
    expect(extractText(flat, range)).toBe(",");
  });
});

describe("line selection (triple-click)", () => {
  test("selects the entire rendered line", () => {
    const lines = [line("first line"), line("second line")];
    const flat = buildFlatIndex(lines);
    const range = lineRangeAt(flat, flat[1]!.start + 3);
    expect(extractText(flat, range)).toBe("second line");
  });
});

describe("drag selection across lines: wrap vs real line breaks", () => {
  test("soft-wrapped lines rejoin with a space when copied", () => {
    const lines = [line("first part", { wrapContinuation: false }), line("second part", { wrapContinuation: true })];
    const flat = buildFlatIndex(lines);
    const range = selectAllRange(flat);
    expect(extractText(flat, range)).toBe("first part second part");
  });

  test("real line breaks (not wrap continuations) rejoin with a newline", () => {
    const lines = [line("item one"), line("item two")];
    const flat = buildFlatIndex(lines);
    const range = selectAllRange(flat);
    expect(extractText(flat, range)).toBe("item one\nitem two");
  });

  test("a real markdown paragraph that wraps to 3 visual lines copies as one prose line", () => {
    const rendered = render("the quick brown fox jumps over the lazy dog and keeps going", 20);
    expect(rendered.length).toBeGreaterThan(1); // actually wrapped
    const flat = buildFlatIndex(rendered);
    const text = extractText(flat, selectAllRange(flat));
    expect(text).toBe("the quick brown fox jumps over the lazy dog and keeps going");
  });
});

describe("trailing whitespace is stripped on copy", () => {
  test("trailing spaces on a line are removed", () => {
    const lines = [line("hello   ")];
    const flat = buildFlatIndex(lines);
    expect(extractText(flat, selectAllRange(flat))).toBe("hello");
  });
});

describe("selection survives scrolling and streaming (content-anchored)", () => {
  test("an offset range computed before streamed text appends still extracts the same original text", () => {
    const before = [line("Explain what you")];
    const flatBefore = buildFlatIndex(before);
    const range = wordRangeAt(flatBefore, 0); // selects "Explain"
    expect(extractText(flatBefore, range)).toBe("Explain");

    // More tokens stream in; existing content is unchanged, only appended to.
    const after = [line("Explain what you are, briefly.")];
    const flatAfter = buildFlatIndex(after);
    // The SAME offset range (computed before the new tokens arrived)
    // still resolves to the same original word, because streaming only
    // appends — it never shifts earlier offsets.
    expect(extractText(flatAfter, range)).toBe("Explain");
  });

  test("offsets are unaffected by which lines are currently scrolled into view", () => {
    const manyLines = Array.from({ length: 200 }, (_, i) => line(`line ${i}`));
    const flat = buildFlatIndex(manyLines);
    // Selecting a line far outside any typical viewport (scroll position is irrelevant to the flat index itself).
    const range = lineRangeAt(flat, flat[150]!.start);
    expect(extractText(flat, range)).toBe("line 150");
  });
});

describe("select all", () => {
  test("covers the full document", () => {
    const lines = [line("a"), line("b"), line("c")];
    const flat = buildFlatIndex(lines);
    expect(extractText(flat, selectAllRange(flat))).toBe("a\nb\nc");
    expect(selectAllRange(flat).end).toBe(totalLength(flat));
  });
});

describe("code block copy", () => {
  test("Copy extracts only this block's lines, real-newline-joined", () => {
    const src = "```js\nconst a = 1;\nconst b = 2;\n```";
    const lines = render(src);
    const codeBlockId = lines.find((l) => l.isCodeBlockHeader)!.codeBlockId!;
    expect(extractCodeBlock(lines, codeBlockId)).toBe("const a = 1;\nconst b = 2;");
  });

  test("code block text never includes the header/footer chrome", () => {
    const src = "```python\nx = 1\n```";
    const lines = render(src);
    const codeBlockId = lines.find((l) => l.isCodeBlockHeader)!.codeBlockId!;
    const copied = extractCodeBlock(lines, codeBlockId);
    expect(copied).not.toContain("python");
  });
});

describe("Copy as Markdown", () => {
  test("preserves original markdown syntax instead of the flattened text", () => {
    const src = "**bold** and *italic*";
    const lines = render(src);
    const flat = buildFlatIndex(lines);
    const markdown = extractMarkdownSource(flat, selectAllRange(flat), src);
    expect(markdown).toBe(src);
  });
});

describe("normalizeRange", () => {
  test("orders start before end regardless of drag direction", () => {
    expect(normalizeRange(10, 3)).toEqual({ start: 3, end: 10 });
    expect(normalizeRange(3, 10)).toEqual({ start: 3, end: 10 });
  });
});
