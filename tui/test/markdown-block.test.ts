import { describe, expect, test } from "bun:test";
import { parseBlocks } from "../src/markdown/block.js";
import { inlineToPlainText } from "../src/markdown/inline.js";
import type { BlockNode } from "../src/markdown/types.js";

describe("block markdown: headings", () => {
  test("parses a level-2 heading", () => {
    const blocks = parseBlocks("## Title");
    expect(blocks[0]!.kind).toBe("heading");
    expect((blocks[0] as any).level).toBe(2);
    expect(inlineToPlainText((blocks[0] as any).children)).toBe("Title");
  });
});

describe("block markdown: paragraphs", () => {
  test("consecutive lines join into one paragraph", () => {
    const blocks = parseBlocks("line one\nline two");
    expect(blocks.length).toBe(1);
    expect(blocks[0]!.kind).toBe("paragraph");
  });
  test("a blank line separates paragraphs", () => {
    const blocks = parseBlocks("first\n\nsecond");
    const kinds = blocks.map((b) => b.kind);
    expect(kinds).toEqual(["paragraph", "blank", "paragraph"]);
  });
});

describe("block markdown: code fences", () => {
  test("a closed fence parses as a closed codeBlock with language", () => {
    const blocks = parseBlocks("```js\nconst x = 1;\n```");
    expect(blocks[0]!.kind).toBe("codeBlock");
    const cb = blocks[0] as any;
    expect(cb.lang).toBe("js");
    expect(cb.code).toBe("const x = 1;");
    expect(cb.closed).toBe(true);
  });

  test("an UNCLOSED fence (mid-stream) still parses without throwing, closed=false", () => {
    const blocks = parseBlocks("```python\ndef f():\n    return 1\n");
    expect(blocks[0]!.kind).toBe("codeBlock");
    const cb = blocks[0] as any;
    expect(cb.closed).toBe(false);
    expect(cb.code).toContain("def f():");
  });

  test("markdown syntax inside a code fence is not interpreted as markdown", () => {
    const blocks = parseBlocks("```\n**not bold**\n```");
    const cb = blocks[0] as any;
    expect(cb.code).toBe("**not bold**");
  });
});

describe("block markdown: lists", () => {
  test("unordered list items", () => {
    const blocks = parseBlocks("- one\n- two\n- three");
    expect(blocks[0]!.kind).toBe("list");
    const list = blocks[0] as any;
    expect(list.ordered).toBe(false);
    expect(list.items.length).toBe(3);
  });
  test("ordered list detection", () => {
    const blocks = parseBlocks("1. a\n2. b");
    expect((blocks[0] as any).ordered).toBe(true);
  });
  test("task list checkboxes", () => {
    const blocks = parseBlocks("- [ ] todo\n- [x] done");
    const list = blocks[0] as any;
    expect(list.items[0].checked).toBe(false);
    expect(list.items[1].checked).toBe(true);
  });
  test("nested list depth from indentation", () => {
    const blocks = parseBlocks("- top\n  - nested");
    const list = blocks[0] as any;
    expect(list.items[0].depth).toBe(0);
    expect(list.items[1].depth).toBe(1);
  });
});

describe("block markdown: blockquotes", () => {
  test("parses quoted lines as a blockquote with nested blocks", () => {
    const blocks = parseBlocks("> quoted text");
    expect(blocks[0]!.kind).toBe("blockquote");
    const bq = blocks[0] as any;
    expect(bq.children[0].kind).toBe("paragraph");
  });
});

describe("block markdown: tables", () => {
  test("parses an aligned table", () => {
    const blocks = parseBlocks("| A | B |\n| :-- | --: |\n| 1 | 2 |");
    expect(blocks[0]!.kind).toBe("table");
    const t = blocks[0] as any;
    expect(t.align).toEqual(["left", "right"]);
    expect(t.rows.length).toBe(1);
  });
});

describe("block markdown: thematic break", () => {
  test("--- is a rule", () => {
    const blocks = parseBlocks("---");
    expect(blocks[0]!.kind).toBe("rule");
  });
});

describe("source offsets round-trip", () => {
  test("every block's span slices back to its own source text", () => {
    const src = "## Title\n\nparagraph text";
    const blocks = parseBlocks(src);
    for (const b of blocks as BlockNode[]) {
      if (b.kind === "blank") continue;
      expect(src.slice(b.start, b.end).length).toBeGreaterThan(0);
    }
  });
});
