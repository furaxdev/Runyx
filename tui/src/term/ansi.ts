// Raw ANSI/VT sequence builders. Kept as plain functions returning
// strings (no terminal state here) so the renderer can unit-test
// sequence generation without a real TTY.
import { type ColorLevel, type RGB, rgbTo16, rgbTo256 } from "./color.js";

export const ESC = "\x1b";
export const CSI = `${ESC}[`;
export const OSC = `${ESC}]`;
export const ST = `${ESC}\\`;

export const ALT_SCREEN_ON = `${CSI}?1049h`;
export const ALT_SCREEN_OFF = `${CSI}?1049l`;
export const CURSOR_HIDE = `${CSI}?25l`;
export const CURSOR_SHOW = `${CSI}?25h`;
export const SYNC_BEGIN = `${CSI}?2026h`;
export const SYNC_END = `${CSI}?2026l`;
export const BRACKETED_PASTE_ON = `${CSI}?2004h`;
export const BRACKETED_PASTE_OFF = `${CSI}?2004l`;
export const FOCUS_REPORT_ON = `${CSI}?1004h`;
export const FOCUS_REPORT_OFF = `${CSI}?1004l`;
// SGR mouse mode (1006) + any-event tracking (1003, implies button-event 1002).
export const MOUSE_ON = `${CSI}?1000h${CSI}?1002h${CSI}?1003h${CSI}?1006h`;
export const MOUSE_OFF = `${CSI}?1006l${CSI}?1003l${CSI}?1002l${CSI}?1000l`;
// Kitty keyboard protocol: push a flag set (disambiguate escape codes +
// report event types), pop on exit. Terminals that don't support it
// ignore unknown CSI sequences harmlessly (no visible effect), which is
// why no capability probe gates sending this — see input.ts for the
// corresponding parser, which falls back to legacy sequences either way.
export const KITTY_KEYBOARD_PUSH = `${CSI}>5u`;
export const KITTY_KEYBOARD_POP = `${CSI}<u`;

export function cursorTo(row: number, col: number): string {
  return `${CSI}${row + 1};${col + 1}H`;
}

export function eraseScreen(): string {
  return `${CSI}2J`;
}

export const RESET_SGR = `${CSI}0m`;

export interface SGRAttrs {
  bold?: boolean;
  italic?: boolean;
  underline?: boolean;
  dim?: boolean;
  strike?: boolean;
  reverse?: boolean;
}

/** Builds one SGR sequence for the given fg/bg/attrs at the given color level. */
export function sgr(fg: RGB | null, bg: RGB | null, attrs: SGRAttrs, level: ColorLevel): string {
  if (level === "none") {
    // Monochrome: attributes still convey meaning (bold/underline/reverse).
    const codes: number[] = [];
    if (attrs.bold) codes.push(1);
    if (attrs.dim) codes.push(2);
    if (attrs.italic) codes.push(3);
    if (attrs.underline) codes.push(4);
    if (attrs.reverse) codes.push(7);
    if (attrs.strike) codes.push(9);
    return codes.length ? `${CSI}${codes.join(";")}m` : "";
  }
  const codes: number[] = [0];
  if (attrs.bold) codes.push(1);
  if (attrs.dim) codes.push(2);
  if (attrs.italic) codes.push(3);
  if (attrs.underline) codes.push(4);
  if (attrs.reverse) codes.push(7);
  if (attrs.strike) codes.push(9);
  if (fg) codes.push(...fgCodes(fg, level));
  if (bg) codes.push(...bgCodes(bg, level));
  return `${CSI}${codes.join(";")}m`;
}

function fgCodes(rgb: RGB, level: ColorLevel): number[] {
  if (level === "truecolor") return [38, 2, rgb[0], rgb[1], rgb[2]];
  if (level === "256") return [38, 5, rgbTo256(rgb)];
  return [rgbTo16(rgb, false)];
}

function bgCodes(rgb: RGB, level: ColorLevel): number[] {
  if (level === "truecolor") return [48, 2, rgb[0], rgb[1], rgb[2]];
  if (level === "256") return [48, 5, rgbTo256(rgb)];
  return [rgbTo16(rgb, true)];
}

/** OSC 8 hyperlink start/end wrapper. */
export function hyperlink(url: string, id: string): { start: string; end: string } {
  return {
    start: `${OSC}8;id=${id};${url}${ST}`,
    end: `${OSC}8;;${ST}`,
  };
}

/** OSC 52 clipboard-set sequence (base64 payload), works over SSH/tmux. */
export function osc52Copy(base64: string): string {
  return `${OSC}52;c;${base64}${ST}`;
}
