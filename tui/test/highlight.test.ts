import { describe, expect, test } from "bun:test";
import { highlightLine } from "../src/markdown/highlight.js";

describe("highlightLine", () => {
  test("JS keyword is tagged", () => {
    const tokens = highlightLine("const x = 1;", "js");
    expect(tokens.some((t) => t.text === "const" && t.kind === "keyword")).toBe(true);
  });
  test("JS string literal is tagged", () => {
    const tokens = highlightLine('const s = "hi";', "js");
    expect(tokens.some((t) => t.kind === "string" && t.text === '"hi"')).toBe(true);
  });
  test("Python keyword", () => {
    const tokens = highlightLine("def foo():", "python");
    expect(tokens.some((t) => t.text === "def" && t.kind === "keyword")).toBe(true);
  });
  test("shell comment", () => {
    const tokens = highlightLine("echo hi # comment", "bash");
    expect(tokens.some((t) => t.kind === "comment")).toBe(true);
  });
  test("unknown language falls back to plain + number/string highlighting", () => {
    const tokens = highlightLine("value = 42", "made-up-lang");
    expect(tokens.some((t) => t.text === "42" && t.kind === "number")).toBe(true);
  });
  test("reconstructs the original line exactly from token text", () => {
    const line = 'const x = "hi"; // done';
    const tokens = highlightLine(line, "js");
    expect(tokens.map((t) => t.text).join("")).toBe(line);
  });
});
