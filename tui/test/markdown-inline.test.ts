import { describe, expect, test } from "bun:test";
import { inlineToPlainText, parseInline } from "../src/markdown/inline.js";

describe("inline markdown: bold alone", () => {
  test("parses **bold** as a single bold node", () => {
    const nodes = parseInline("hello **world** end");
    expect(nodes.map((n) => n.kind)).toEqual(["text", "bold", "text"]);
    expect(inlineToPlainText(nodes)).toBe("hello world end");
  });
  test("unterminated ** stays literal text (streaming-safe)", () => {
    const nodes = parseInline("hello **world");
    expect(nodes.every((n) => n.kind === "text")).toBe(true);
    expect(inlineToPlainText(nodes)).toBe("hello **world");
  });
});

describe("inline markdown: italic alone", () => {
  test("parses *italic* as a single italic node", () => {
    const nodes = parseInline("hello *world* end");
    expect(nodes.map((n) => n.kind)).toEqual(["text", "italic", "text"]);
  });
  test("parses _italic_ (underscore form)", () => {
    const nodes = parseInline("a _b_ c");
    expect(nodes[1]!.kind).toBe("italic");
  });
  test("unterminated * stays literal text", () => {
    const nodes = parseInline("hello *world");
    expect(nodes.every((n) => n.kind === "text")).toBe(true);
  });
});

describe("inline markdown: bold+italic combined", () => {
  test("***text*** parses as boldItalic", () => {
    const nodes = parseInline("say ***wow*** now");
    expect(nodes.map((n) => n.kind)).toEqual(["text", "boldItalic", "text"]);
    expect(inlineToPlainText(nodes)).toBe("say wow now");
  });
  test("italic nested inside bold", () => {
    const nodes = parseInline("**bold *and italic* text**");
    expect(nodes.length).toBe(1);
    expect(nodes[0]!.kind).toBe("bold");
    const children = (nodes[0] as any).children;
    expect(children.some((c: any) => c.kind === "italic")).toBe(true);
  });
});

describe("inline markdown: other forms", () => {
  test("strikethrough", () => {
    const nodes = parseInline("~~gone~~");
    expect(nodes[0]!.kind).toBe("strike");
  });
  test("inline code is not parsed as markdown inside", () => {
    const nodes = parseInline("run `**not bold**` now");
    const code = nodes.find((n) => n.kind === "code");
    expect(code).toBeDefined();
    expect((code as any).text).toBe("**not bold**");
  });
  test("link", () => {
    const nodes = parseInline("see [docs](https://example.com/x) now");
    const link = nodes.find((n) => n.kind === "link") as any;
    expect(link.text).toBe("docs");
    expect(link.url).toBe("https://example.com/x");
  });
  test("plain text with no markdown stays a single text node", () => {
    const nodes = parseInline("just plain text");
    expect(nodes).toEqual([{ kind: "text", text: "just plain text", start: 0, end: 15 }]);
  });
});

describe("source offsets (selection anchoring)", () => {
  test("node offsets map back into the original source string", () => {
    const src = "hello **world** end";
    const nodes = parseInline(src);
    const bold = nodes.find((n) => n.kind === "bold")!;
    expect(src.slice(bold.start, bold.end)).toBe("**world**");
  });
  test("offsets respect a non-zero baseOffset", () => {
    const nodes = parseInline("**x**", 100);
    expect(nodes[0]!.start).toBe(100);
    expect(nodes[0]!.end).toBe(105);
  });
});
