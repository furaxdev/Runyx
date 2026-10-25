import { describe, expect, test } from "bun:test";
import { layoutFlex } from "../src/layout/flex.js";

describe("layoutFlex", () => {
  test("splits fixed-size columns left to right", () => {
    const out = layoutFlex({ row: 0, col: 0, width: 30, height: 10 }, "row", [
      { size: 10, data: "a" },
      { size: 20, data: "b" },
    ]);
    expect(out[0]!.rect).toEqual({ row: 0, col: 0, width: 10, height: 10 });
    expect(out[1]!.rect).toEqual({ row: 0, col: 10, width: 20, height: 10 });
  });

  test("flex item takes the remaining space", () => {
    const out = layoutFlex({ row: 0, col: 0, width: 30, height: 10 }, "row", [
      { size: 10, data: "fixed" },
      { size: { flex: 1 }, data: "flex" },
    ]);
    expect(out[1]!.rect.width).toBe(20);
  });

  test("multiple flex items share proportionally by weight", () => {
    const out = layoutFlex({ row: 0, col: 0, width: 30, height: 10 }, "row", [
      { size: { flex: 1 }, data: "a" },
      { size: { flex: 2 }, data: "b" },
    ]);
    expect(out[0]!.rect.width).toBe(10);
    expect(out[1]!.rect.width).toBe(20);
  });

  test("column direction stacks vertically", () => {
    const out = layoutFlex({ row: 0, col: 0, width: 10, height: 20 }, "column", [
      { size: 5, data: "a" },
      { size: { flex: 1 }, data: "b" },
    ]);
    expect(out[0]!.rect).toEqual({ row: 0, col: 0, width: 10, height: 5 });
    expect(out[1]!.rect.height).toBe(15);
  });

  test("gap is subtracted from distributable space", () => {
    const out = layoutFlex({ row: 0, col: 0, width: 30, height: 10 }, "row", [
      { size: { flex: 1 }, data: "a" },
      { size: { flex: 1 }, data: "b" },
    ], { gap: 2 });
    // 30 - 2 gap = 28, split evenly -> 14 each
    expect(out[0]!.rect.width).toBe(14);
    expect(out[1]!.rect.width).toBe(14);
    expect(out[1]!.rect.col).toBe(16); // 0 + 14 + 2 gap
  });

  test("padding insets the whole layout", () => {
    const out = layoutFlex({ row: 0, col: 0, width: 20, height: 20 }, "row", [{ size: { flex: 1 }, data: "a" }], { padding: 2 });
    expect(out[0]!.rect).toEqual({ row: 2, col: 2, width: 16, height: 16 });
  });

  test("sizes sum exactly to the available space with rounding", () => {
    const out = layoutFlex({ row: 0, col: 0, width: 10, height: 1 }, "row", [
      { size: { flex: 1 }, data: "a" },
      { size: { flex: 1 }, data: "b" },
      { size: { flex: 1 }, data: "c" },
    ]);
    const total = out.reduce((sum, o) => sum + o.rect.width, 0);
    expect(total).toBe(10);
  });
});
