// Text drawing: width-aware word wrap (counts display width, not UTF-16
// length or code points, so CJK/emoji wrap correctly) and a draw
// helper used by every text-bearing widget.
import type { Screen, Cell } from "../term/buffer.js";
import { graphemes, stringWidth } from "../term/width.js";
import type { Rect } from "../layout/flex.js";

/** Wraps text to `width` cells, breaking on spaces and hard-breaking a single word longer than width. */
export function wrapText(text: string, width: number): string[] {
  if (width <= 0) return [text];
  const lines: string[] = [];
  for (const paragraph of text.split("\n")) {
    if (paragraph === "") {
      lines.push("");
      continue;
    }
    let current = "";
    let currentWidth = 0;
    const words = paragraph.split(/(\s+)/);
    for (const word of words) {
      const w = stringWidth(word);
      if (currentWidth + w <= width) {
        current += word;
        currentWidth += w;
        continue;
      }
      if (current.trim() !== "") {
        lines.push(current.replace(/\s+$/, ""));
      }
      current = "";
      currentWidth = 0;
      if (w > width) {
        // Hard-break a word longer than the whole line width.
        let piece = "";
        let pieceWidth = 0;
        for (const g of graphemes(word)) {
          if (pieceWidth + g.width > width) {
            lines.push(piece);
            piece = "";
            pieceWidth = 0;
          }
          piece += g.text;
          pieceWidth += g.width;
        }
        current = piece;
        currentWidth = pieceWidth;
      } else if (word.trim() !== "") {
        current = word;
        currentWidth = w;
      }
    }
    lines.push(current.replace(/\s+$/, ""));
  }
  return lines;
}

export interface TextStyle extends Partial<Omit<Cell, "text" | "width">> {}

/** Draws a single already-wrapped line, clipped to rect.width, truncating with an ellipsis if `ellipsis` is set and it overflows. */
export function drawLine(screen: Screen, rect: Rect, row: number, line: string, style: TextStyle = {}, ellipsis = false): void {
  if (row < rect.row || row >= rect.row + rect.height) return;
  const w = stringWidth(line);
  if (ellipsis && w > rect.width && rect.width > 1) {
    let acc = "";
    let accWidth = 0;
    for (const g of graphemes(line)) {
      if (accWidth + g.width > rect.width - 1) break;
      acc += g.text;
      accWidth += g.width;
    }
    screen.writeText(row, rect.col, acc + "…", style);
    return;
  }
  screen.writeText(row, rect.col, line, style);
}

/** Draws wrapped text within rect, top-aligned, clipped to rect.height. Returns the number of lines the text wrapped to (for layout). */
export function drawWrappedText(screen: Screen, rect: Rect, text: string, style: TextStyle = {}): number {
  const lines = wrapText(text, rect.width);
  for (let i = 0; i < lines.length && i < rect.height; i++) {
    drawLine(screen, rect, rect.row + i, lines[i]!, style);
  }
  return lines.length;
}
