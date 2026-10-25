import { describe, expect, test } from "bun:test";
import { Screen } from "../src/term/buffer.js";

describe("Screen diff rendering", () => {
  test("first diff after fullRepaint draws every written cell", () => {
    const s = new Screen(10, 2);
    s.clear();
    s.writeText(0, 0, "hi", { fg: null, bg: null, attrs: {}, linkId: "", linkUrl: "", zoneId: 0, selectable: false });
    const out = s.diff("truecolor", false);
    expect(out).toContain("h");
    expect(out).toContain("i");
  });

  test("unchanged cells produce no diff output on the second frame", () => {
    const s = new Screen(10, 2);
    s.clear();
    s.writeText(0, 0, "hi", {});
    s.diff("truecolor", false); // commit frame 1

    s.clear();
    s.writeText(0, 0, "hi", {}); // identical content
    const out = s.diff("truecolor", false);
    expect(out).toBe("");
  });

  test("only the changed cell is redrawn", () => {
    const s = new Screen(10, 2);
    s.clear();
    s.writeText(0, 0, "aaa", {});
    s.diff("truecolor", false);

    s.clear();
    s.writeText(0, 0, "aXa", {});
    const out = s.diff("truecolor", false);
    expect(out).toContain("X");
    // Should not need to rewrite the unchanged 'a's as part of this run
    // (a cursor move directly to the changed column, not from column 0).
    expect(out).not.toMatch(/;1Haaa/);
  });

  test("wide characters occupy two cells and only the lead cell carries text", () => {
    const s = new Screen(5, 1);
    s.clear();
    s.writeText(0, 0, "中", {});
    expect(s.backAt(0, 0)?.width).toBe(2);
    expect(s.backAt(0, 0)?.text).toBe("中");
    expect(s.backAt(0, 1)?.width).toBe(0);
  });

  test("resize clears both buffers to blank", () => {
    const s = new Screen(5, 1);
    s.clear();
    s.writeText(0, 0, "hi", {});
    s.diff("truecolor", false);
    s.resize(8, 2);
    expect(s.width).toBe(8);
    expect(s.height).toBe(2);
    expect(s.frontAt(0, 0)?.text).toBe(" ");
  });

  test("fillRect fills the whole region", () => {
    const s = new Screen(4, 2);
    s.clear();
    s.fillRect(0, 0, 4, 2, "#", {});
    for (let r = 0; r < 2; r++) for (let c = 0; c < 4; c++) expect(s.backAt(r, c)?.text).toBe("#");
  });

  test("text clips at the right edge instead of wrapping", () => {
    const s = new Screen(3, 1);
    s.clear();
    const endCol = s.writeText(0, 0, "hello", {});
    expect(endCol).toBe(3);
    expect(s.backAt(0, 2)?.text).toBe("l");
  });
});
