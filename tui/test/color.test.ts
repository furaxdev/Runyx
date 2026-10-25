import { describe, expect, test } from "bun:test";
import { detectColorLevel, rgbTo16, rgbTo256 } from "../src/term/color.js";

describe("detectColorLevel", () => {
  test("NO_COLOR disables color entirely", () => {
    expect(detectColorLevel({ NO_COLOR: "1", TERM: "xterm-256color" }, true)).toBe("none");
  });
  test("non-TTY output disables color", () => {
    expect(detectColorLevel({ COLORTERM: "truecolor" }, false)).toBe("none");
  });
  test("COLORTERM=truecolor wins", () => {
    expect(detectColorLevel({ COLORTERM: "truecolor" }, true)).toBe("truecolor");
  });
  test("256color TERM without COLORTERM", () => {
    expect(detectColorLevel({ TERM: "xterm-256color" }, true)).toBe("256");
  });
  test("plain TERM falls back to 16 colors", () => {
    expect(detectColorLevel({ TERM: "xterm" }, true)).toBe("16");
  });
  test("dumb TERM disables color", () => {
    expect(detectColorLevel({ TERM: "dumb" }, true)).toBe("none");
  });
});

describe("rgbTo256", () => {
  test("pure black maps to the grayscale ramp start", () => {
    expect(rgbTo256([0, 0, 0])).toBe(16);
  });
  test("pure white maps to the grayscale ramp end", () => {
    expect(rgbTo256([255, 255, 255])).toBe(231);
  });
  test("saturated red maps into the 6x6x6 color cube", () => {
    const idx = rgbTo256([255, 0, 0]);
    expect(idx).toBeGreaterThanOrEqual(16);
    expect(idx).toBeLessThanOrEqual(231);
  });
});

describe("rgbTo16", () => {
  test("picks the nearest of the 16 ANSI colors", () => {
    const idx = rgbTo16([200, 50, 50], false);
    expect(idx).toBeGreaterThanOrEqual(30);
    expect(idx).toBeLessThanOrEqual(97);
  });
  test("background variant adds 10 (or uses bright-bg range)", () => {
    const fg = rgbTo16([0, 0, 0], false);
    const bg = rgbTo16([0, 0, 0], true);
    expect(bg).toBe(fg + 10);
  });
});
