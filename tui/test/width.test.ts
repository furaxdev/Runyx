import { describe, expect, test } from "bun:test";
import { codePointWidth, graphemes, stringWidth } from "../src/term/width.js";

describe("codePointWidth", () => {
  test("ASCII is width 1", () => {
    expect(codePointWidth("a".codePointAt(0)!)).toBe(1);
  });
  test("CJK ideographs are width 2", () => {
    expect(codePointWidth("中".codePointAt(0)!)).toBe(2);
    expect(codePointWidth("日".codePointAt(0)!)).toBe(2);
  });
  test("Hangul syllables are width 2", () => {
    expect(codePointWidth("한".codePointAt(0)!)).toBe(2);
  });
  test("combining accent is width 0", () => {
    expect(codePointWidth(0x0301)).toBe(0); // combining acute accent
  });
  test("control characters are width 0", () => {
    expect(codePointWidth(0x0001)).toBe(0);
  });
});

describe("graphemes", () => {
  test("splits plain ASCII into single-width clusters", () => {
    const g = graphemes("abc");
    expect(g.map((x) => x.text)).toEqual(["a", "b", "c"]);
    expect(g.every((x) => x.width === 1)).toBe(true);
  });
  test("keeps a base char + combining accent as one width-1 cluster", () => {
    const g = graphemes("éllo"); // e + combining acute = é
    expect(g[0]!.text).toBe("é");
    expect(g[0]!.width).toBe(1);
  });
  test("CJK string produces width-2 clusters", () => {
    const g = graphemes("中文");
    expect(g.length).toBe(2);
    expect(g[0]!.width).toBe(2);
    expect(g[1]!.width).toBe(2);
  });
  test("mixed ASCII + CJK total width", () => {
    expect(stringWidth("ab中")).toBe(4); // 1+1+2
  });
  test("empty string yields no clusters", () => {
    expect(graphemes("")).toEqual([]);
  });
});
