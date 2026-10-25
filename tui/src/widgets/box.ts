// Bordered box drawing (single-line box-drawing characters, with an
// ASCII fallback for --plain/no-unicode contexts) and background fill.
import type { Screen, Cell } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import type { RGB } from "../term/color.js";

export interface BoxChars {
  tl: string;
  tr: string;
  bl: string;
  br: string;
  h: string;
  v: string;
}
export const UNICODE_BOX: BoxChars = { tl: "╭", tr: "╮", bl: "╰", br: "╯", h: "─", v: "│" };
export const ASCII_BOX: BoxChars = { tl: "+", tr: "+", bl: "+", br: "+", h: "-", v: "|" };

export interface BoxStyle {
  fg?: RGB | null;
  bg?: RGB | null;
  borderFg?: RGB | null;
  title?: string;
  chars?: BoxChars;
}

export function drawBox(screen: Screen, rect: Rect, style: BoxStyle = {}): Rect {
  const chars = style.chars ?? UNICODE_BOX;
  const borderStyle: Partial<Omit<Cell, "text" | "width">> = { fg: style.borderFg ?? null, bg: style.bg ?? null };
  if (style.bg) screen.fillRect(rect.row, rect.col, rect.width, rect.height, " ", { bg: style.bg });

  if (rect.width >= 2 && rect.height >= 2) {
    screen.setCluster(rect.row, rect.col, chars.tl, 1, borderStyle);
    screen.setCluster(rect.row, rect.col + rect.width - 1, chars.tr, 1, borderStyle);
    screen.setCluster(rect.row + rect.height - 1, rect.col, chars.bl, 1, borderStyle);
    screen.setCluster(rect.row + rect.height - 1, rect.col + rect.width - 1, chars.br, 1, borderStyle);
    for (let c = rect.col + 1; c < rect.col + rect.width - 1; c++) {
      screen.setCluster(rect.row, c, chars.h, 1, borderStyle);
      screen.setCluster(rect.row + rect.height - 1, c, chars.h, 1, borderStyle);
    }
    for (let r = rect.row + 1; r < rect.row + rect.height - 1; r++) {
      screen.setCluster(r, rect.col, chars.v, 1, borderStyle);
      screen.setCluster(r, rect.col + rect.width - 1, chars.v, 1, borderStyle);
    }
    if (style.title) {
      const title = ` ${style.title} `;
      screen.writeText(rect.row, rect.col + 2, title, { fg: style.fg ?? null, bg: style.bg ?? null });
    }
  }
  return { row: rect.row + 1, col: rect.col + 1, width: Math.max(0, rect.width - 2), height: Math.max(0, rect.height - 2) };
}

export function fillBg(screen: Screen, rect: Rect, bg: RGB): void {
  screen.fillRect(rect.row, rect.col, rect.width, rect.height, " ", { bg });
}
