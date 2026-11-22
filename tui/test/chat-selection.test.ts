import { describe, expect, test } from "bun:test";
import { clampSelectionToLine } from "../src/screens/chat.js";
import type { FlatLine } from "../src/selection/selection.js";

function flatLine(lineIndex: number, start: number, end: number): FlatLine {
  return { lineIndex, start, end, line: { spans: [], plainText: "x".repeat(end - start), source: null } };
}

describe("clampSelectionToLine", () => {
  test("selection entirely before the line: no overlap", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 0, end: 5 }, f)).toBeNull();
  });

  test("selection entirely after the line: no overlap", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 25, end: 30 }, f)).toBeNull();
  });

  test("selection fully covers the line", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 0, end: 30 }, f)).toEqual({ start: 0, end: 10 });
  });

  test("selection starts mid-line and extends past the end", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 15, end: 30 }, f)).toEqual({ start: 5, end: 10 });
  });

  test("selection starts before and ends mid-line", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 0, end: 15 }, f)).toEqual({ start: 0, end: 5 });
  });

  test("selection wholly inside the line", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 12, end: 17 }, f)).toEqual({ start: 2, end: 7 });
  });

  test("zero-width (collapsed) selection never highlights anything", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 14, end: 14 }, f)).toBeNull();
  });

  test("touching exactly at the line's end boundary: no overlap (end is exclusive)", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 20, end: 25 }, f)).toBeNull();
  });

  test("touching exactly at the line's start boundary: no overlap", () => {
    const f = flatLine(1, 10, 20);
    expect(clampSelectionToLine({ start: 0, end: 10 }, f)).toBeNull();
  });

  test("zero-length line never produces a visible highlight", () => {
    const f = flatLine(0, 5, 5);
    expect(clampSelectionToLine({ start: 0, end: 30 }, f)).toBeNull();
  });
});
