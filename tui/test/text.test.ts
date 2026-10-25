import { describe, expect, test } from "bun:test";
import { wrapText } from "../src/widgets/text.js";

describe("wrapText", () => {
  test("short text fits on one line", () => {
    expect(wrapText("hello", 20)).toEqual(["hello"]);
  });

  test("wraps on word boundaries", () => {
    const lines = wrapText("the quick brown fox", 10);
    for (const l of lines) expect(l.length).toBeLessThanOrEqual(10);
    expect(lines.join(" ")).toBe("the quick brown fox");
  });

  test("hard-breaks a single word longer than the width", () => {
    const lines = wrapText("supercalifragilisticexpialidocious", 10);
    expect(lines.length).toBeGreaterThan(1);
    for (const l of lines) expect(l.length).toBeLessThanOrEqual(10);
  });

  test("preserves explicit newlines as paragraph breaks", () => {
    const lines = wrapText("line one\nline two", 20);
    expect(lines).toEqual(["line one", "line two"]);
  });

  test("empty paragraphs (blank lines) are preserved", () => {
    const lines = wrapText("a\n\nb", 20);
    expect(lines).toEqual(["a", "", "b"]);
  });

  test("CJK text wraps by display width, not character count", () => {
    // Each CJK char is width 2, so width=10 should fit 5 of them.
    const lines = wrapText("中文中文中文中文中文中文", 10);
    for (const l of lines) {
      const w = Array.from(l).length * 2;
      expect(w).toBeLessThanOrEqual(10);
    }
  });
});
