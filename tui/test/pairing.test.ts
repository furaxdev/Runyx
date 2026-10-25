import { describe, expect, test } from "bun:test";
import { Screen } from "../src/term/buffer.js";
import { drawQRCode } from "../src/screens/pairing.js";

const DARK: [number, number, number] = [0, 0, 0];
const LIGHT: [number, number, number] = [255, 255, 255];

describe("drawQRCode", () => {
  test("packs two module rows into one terminal row using half-block characters", () => {
    const screen = new Screen(4, 2);
    const modules = [
      [true, false, true, false],
      [false, true, false, true],
    ];
    drawQRCode(screen, { row: 0, col: 0, width: 4, height: 1 }, modules, DARK, LIGHT);

    // Column 0: top=true, bottom=false -> upper half block "▀"
    expect(screen.backAt(0, 0)?.text).toBe("▀");
    // Column 1: top=false, bottom=true -> lower half block "▄"
    expect(screen.backAt(0, 1)?.text).toBe("▄");
    // Column 2: same pattern as column 0
    expect(screen.backAt(0, 2)?.text).toBe("▀");
    // Column 3: same pattern as column 1
    expect(screen.backAt(0, 3)?.text).toBe("▄");
  });

  test("both modules dark renders a full block, both light renders a space", () => {
    const screen = new Screen(2, 2);
    const modules = [
      [true, false],
      [true, false],
    ];
    drawQRCode(screen, { row: 0, col: 0, width: 2, height: 1 }, modules, DARK, LIGHT);
    expect(screen.backAt(0, 0)?.text).toBe("█");
    expect(screen.backAt(0, 1)?.text).toBe(" ");
  });

  test("an odd number of module rows pads the last terminal row with the top half only", () => {
    const screen = new Screen(1, 2);
    const modules = [[true]]; // single row, no partner row below
    drawQRCode(screen, { row: 0, col: 0, width: 1, height: 1 }, modules, DARK, LIGHT);
    expect(screen.backAt(0, 0)?.text).toBe("▀");
  });
});
