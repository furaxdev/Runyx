import { describe, expect, test } from "bun:test";
import { autoscrollDelta, clampScroll, scrollBy, scrollToEnd, visibleRange, type RowHeights } from "../src/layout/scroll.js";

function uniformRows(count: number): RowHeights {
  return { count, heightOf: () => 1 };
}

describe("virtualized scrolling", () => {
  test("content shorter than viewport never scrolls", () => {
    const rows = uniformRows(5);
    const s = clampScroll({ topIndex: 3, topSkip: 0 }, rows, 10);
    expect(s).toEqual({ topIndex: 0, topSkip: 0 });
  });

  test("visibleRange only returns rows that fit the viewport (virtualization)", () => {
    const rows = uniformRows(10000);
    const visible = visibleRange({ topIndex: 5000, topSkip: 0 }, rows, 20);
    expect(visible.length).toBe(20);
    expect(visible[0]!.index).toBe(5000);
    expect(visible[19]!.index).toBe(5019);
  });

  test("clampScroll never scrolls past the point where the last row is visible", () => {
    const rows = uniformRows(100);
    const s = clampScroll({ topIndex: 99, topSkip: 0 }, rows, 10);
    expect(s.topIndex).toBe(90); // last 10 rows (90..99) fill the viewport exactly
  });

  test("scrollToEnd lands on the last page", () => {
    const rows = uniformRows(100);
    const s = scrollToEnd(rows, 10);
    const visible = visibleRange(s, rows, 10);
    expect(visible[visible.length - 1]!.index).toBe(99);
  });

  test("scrollBy moves forward and backward, clamped at both ends", () => {
    const rows = uniformRows(50);
    let s = clampScroll({ topIndex: 0, topSkip: 0 }, rows, 10);
    s = scrollBy(s, rows, 10, 5);
    expect(s.topIndex).toBe(5);
    s = scrollBy(s, rows, 10, -100);
    expect(s.topIndex).toBe(0);
    s = scrollBy(s, rows, 10, 1000);
    expect(s.topIndex).toBe(40); // clamped to last page
  });

  test("variable row heights (wrapped lines) are respected", () => {
    const rows: RowHeights = { count: 3, heightOf: (i) => (i === 1 ? 3 : 1) };
    const visible = visibleRange({ topIndex: 0, topSkip: 0 }, rows, 10);
    expect(visible).toEqual([
      { index: 0, screenRow: 0, rowHeight: 1 },
      { index: 1, screenRow: 1, rowHeight: 3 },
      { index: 2, screenRow: 4, rowHeight: 1 },
    ]);
  });
});

describe("autoscrollDelta", () => {
  test("no autoscroll in the middle of the viewport", () => {
    expect(autoscrollDelta(10, 0, 20, 2)).toBe(0);
  });
  test("negative delta near the top edge", () => {
    expect(autoscrollDelta(0, 0, 20, 2)).toBeLessThan(0);
  });
  test("positive delta near the bottom edge", () => {
    expect(autoscrollDelta(19, 0, 20, 2)).toBeGreaterThan(0);
  });
});
