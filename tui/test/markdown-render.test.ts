import { describe, expect, test } from "bun:test";
import { parseBlocks } from "../src/markdown/block.js";
import { renderMarkdown } from "../src/markdown/render.js";

function render(src: string, width = 40) {
  return renderMarkdown(parseBlocks(src), { width, italicSupported: true });
}

describe("renderMarkdown", () => {
  test("paragraph wraps to the given width", () => {
    const lines = render("the quick brown fox jumps over the lazy dog", 20);
    for (const l of lines) {
      const w = l.spans.reduce((s, sp) => s + sp.text.length, 0);
      expect(w).toBeLessThanOrEqual(20);
    }
  });

  test("bold span carries bold:true", () => {
    const lines = render("**bold text**", 40);
    const boldSpan = lines[0]!.spans.find((s) => s.text.includes("bold"));
    expect(boldSpan?.bold).toBe(true);
  });

  test("italic falls back to underline when the terminal doesn't support italics", () => {
    const lines = renderMarkdown(parseBlocks("*italic*"), { width: 40, italicSupported: false });
    const span = lines[0]!.spans.find((s) => s.text.includes("italic"));
    expect(span?.italic).toBeUndefined();
    expect(span?.underline).toBe(true);
  });

  test("italic uses real italic when supported", () => {
    const lines = render("*italic*", 40);
    const span = lines[0]!.spans.find((s) => s.text.includes("italic"));
    expect(span?.italic).toBe(true);
  });

  test("code block produces a header line and per-line token spans", () => {
    const lines = render("```js\nconst x = 1;\n```", 40);
    expect(lines.some((l) => l.isCodeBlockHeader)).toBe(true);
    const codeLine = lines.find((l) => l.plainText === "const x = 1;");
    expect(codeLine).toBeDefined();
    expect(codeLine!.spans.some((s) => s.colorToken === "keyword")).toBe(true);
  });

  test("unclosed code block has no footer line (still streaming)", () => {
    const lines = render("```js\nconst x = 1;", 40);
    expect(lines.some((l) => l.isCodeBlockFooter)).toBe(false);
  });

  test("every non-blank rendered line keeps a source span into the original text", () => {
    const src = "## Title\n\nSome paragraph text here.";
    const lines = render(src, 40);
    for (const l of lines) {
      if (l.plainText === "") continue;
      expect(l.source).not.toBeNull();
    }
  });

  test("link span carries its URL", () => {
    const lines = render("see [docs](https://x.test)", 40);
    const span = lines[0]!.spans.find((s) => s.linkUrl);
    expect(span?.linkUrl).toBe("https://x.test");
  });

  test("table renders aligned columns with a separator row", () => {
    const lines = render("| A | B |\n| --- | ---: |\n| 1 | 22 |", 40);
    expect(lines.length).toBe(3);
    expect(lines[1]!.spans[0]!.text).toContain("─");
  });

  test("task list items are struck through when checked", () => {
    const lines = render("- [x] done\n- [ ] todo", 40);
    const doneLine = lines.find((l) => l.plainText.includes("done"));
    expect(doneLine!.spans.some((s) => s.strike)).toBe(true);
  });
});
