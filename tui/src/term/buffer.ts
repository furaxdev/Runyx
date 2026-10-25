// The double-buffered cell grid and its differential renderer: the
// foundation everything else (layout, widgets, selection) draws onto.
import { type ColorLevel, type RGB } from "./color.js";
import { CURSOR_HIDE, RESET_SGR, SYNC_BEGIN, SYNC_END, cursorTo, hyperlink, sgr, type SGRAttrs } from "./ansi.js";
import { graphemes } from "./width.js";

export interface Cell {
  /** Grapheme cluster text; "" marks the trailing cell of a wide (width-2) cluster. */
  text: string;
  /** 0 = continuation cell (no text, don't redraw independently), 1 or 2 = cell width. */
  width: 0 | 1 | 2;
  fg: RGB | null;
  bg: RGB | null;
  attrs: SGRAttrs;
  /** OSC 8 hyperlink id (""=none) and the URL it points at. */
  linkId: string;
  linkUrl: string;
  /** Opaque id a widget assigns so mouse hit-testing can map a cell back to it. */
  zoneId: number;
  /** Whether mouse-drag selection may include this cell (false for UI chrome like borders). */
  selectable: boolean;
}

export function blankCell(): Cell {
  return {
    text: " ",
    width: 1,
    fg: null,
    bg: null,
    attrs: {},
    linkId: "",
    linkUrl: "",
    zoneId: 0,
    selectable: false,
  };
}

function cellsEqual(a: Cell, b: Cell): boolean {
  return (
    a.text === b.text &&
    a.width === b.width &&
    rgbEq(a.fg, b.fg) &&
    rgbEq(a.bg, b.bg) &&
    attrsEq(a.attrs, b.attrs) &&
    a.linkId === b.linkId
  );
}
function rgbEq(a: RGB | null, b: RGB | null): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  return a[0] === b[0] && a[1] === b[1] && a[2] === b[2];
}
function attrsEq(a: SGRAttrs, b: SGRAttrs): boolean {
  return !!a.bold === !!b.bold && !!a.italic === !!b.italic && !!a.underline === !!b.underline && !!a.dim === !!b.dim && !!a.strike === !!b.strike && !!a.reverse === !!b.reverse;
}

/**
 * A fixed-size grid of cells with a front (on-screen) and back
 * (being-drawn) buffer. Widgets write into the back buffer via
 * `set`/`text`; `diff()` compares back against front and returns the
 * minimal ANSI string to bring the terminal up to date, then the two
 * buffers swap.
 */
export class Screen {
  width: number;
  height: number;
  private front: Cell[];
  private back: Cell[];

  constructor(width: number, height: number) {
    this.width = Math.max(0, width);
    this.height = Math.max(0, height);
    this.front = this.makeGrid();
    this.back = this.makeGrid();
  }

  private makeGrid(): Cell[] {
    const n = this.width * this.height;
    const grid = new Array<Cell>(n);
    for (let i = 0; i < n; i++) grid[i] = blankCell();
    return grid;
  }

  resize(width: number, height: number): void {
    this.width = Math.max(0, width);
    this.height = Math.max(0, height);
    this.front = this.makeGrid();
    this.back = this.makeGrid();
  }

  /** Clears the back buffer to blank cells (call once per frame before drawing). */
  clear(): void {
    for (let i = 0; i < this.back.length; i++) this.back[i] = blankCell();
  }

  inBounds(row: number, col: number): boolean {
    return row >= 0 && row < this.height && col >= 0 && col < this.width;
  }

  backAt(row: number, col: number): Cell | undefined {
    if (!this.inBounds(row, col)) return undefined;
    return this.back[row * this.width + col];
  }

  frontAt(row: number, col: number): Cell | undefined {
    if (!this.inBounds(row, col)) return undefined;
    return this.front[row * this.width + col];
  }

  /**
   * Writes a single already-measured grapheme cluster at (row, col)
   * into the back buffer, including its trailing continuation cell if
   * width is 2. Silently clips at the grid edge.
   */
  setCluster(row: number, col: number, text: string, width: 1 | 2, style: Partial<Omit<Cell, "text" | "width">>): void {
    if (!this.inBounds(row, col)) return;
    const idx = row * this.width + col;
    const base = this.back[idx]!;
    this.back[idx] = { ...base, ...style, text, width };
    if (width === 2 && this.inBounds(row, col + 1)) {
      const nIdx = row * this.width + col + 1;
      const n = this.back[nIdx]!;
      this.back[nIdx] = { ...n, ...style, text: "", width: 0 };
    }
  }

  /**
   * Writes a plain string starting at (row, col), left to right,
   * stopping at the grid's right edge (never wraps). Returns the column
   * just past the last cell written, for chaining.
   */
  writeText(row: number, col: number, text: string, style: Partial<Omit<Cell, "text" | "width">> = {}): number {
    let c = col;
    for (const g of graphemes(text)) {
      if (c >= this.width) break;
      this.setCluster(row, c, g.text, g.width, style);
      c += g.width;
    }
    return c;
  }

  /** Fills a rectangular region with a single character (e.g. background fill). */
  fillRect(row: number, col: number, w: number, h: number, ch: string, style: Partial<Omit<Cell, "text" | "width">> = {}): void {
    for (let r = row; r < row + h; r++) {
      for (let c = col; c < col + w; c++) {
        this.setCluster(r, c, ch, 1, style);
      }
    }
  }

  /**
   * Compares back against front cell-by-cell, emits the minimal ANSI to
   * bring the terminal in sync (grouping consecutive same-row changes
   * into one cursor move + run of text, reusing the active SGR state so
   * runs of identically-styled cells don't re-emit SGR per cell), then
   * swaps back into front. Wrapped in synchronized-output markers when
   * `sync` is true so a slow terminal never shows a half-drawn frame.
   */
  diff(level: ColorLevel, sync: boolean): string {
    let out = "";
    let lastFg: RGB | null | undefined;
    let lastBg: RGB | null | undefined;
    let lastAttrs: SGRAttrs | undefined;
    let lastLinkId = "";
    let cursorRow = -1;
    let cursorCol = -1;

    const flushLink = (cell: Cell) => {
      if (cell.linkId !== lastLinkId) {
        if (lastLinkId !== "") out += hyperlink("", lastLinkId).end;
        if (cell.linkId !== "") out += hyperlink(cell.linkUrl, cell.linkId).start;
        lastLinkId = cell.linkId;
      }
    };

    for (let row = 0; row < this.height; row++) {
      let col = 0;
      while (col < this.width) {
        const idx = row * this.width + col;
        const f = this.front[idx]!;
        const b = this.back[idx]!;
        if (cellsEqual(f, b)) {
          col++;
          continue;
        }
        if (cursorRow !== row || cursorCol !== col) {
          out += cursorTo(row, col);
        }
        // Emit a run of changed cells in this row at minimal SGR cost.
        let c = col;
        while (c < this.width) {
          const i = row * this.width + c;
          const fc = this.front[i]!;
          const bc = this.back[i]!;
          if (cellsEqual(fc, bc) && c !== col) break;
          if (bc.width === 0) {
            c++;
            continue;
          }
          if (!sameStyle(bc, lastFg, lastBg, lastAttrs)) {
            out += sgr(bc.fg, bc.bg, bc.attrs, level);
            lastFg = bc.fg;
            lastBg = bc.bg;
            lastAttrs = bc.attrs;
          }
          flushLink(bc);
          out += bc.text === "" ? " " : bc.text;
          c += bc.width || 1;
        }
        cursorRow = row;
        cursorCol = c;
        col = c;
      }
    }
    if (lastLinkId !== "") out += hyperlink("", lastLinkId).end;
    if (out !== "") out += RESET_SGR;

    // Swap.
    const tmp = this.front;
    this.front = this.back;
    this.back = tmp;

    if (out === "") return "";
    return sync ? SYNC_BEGIN + out + SYNC_END : out;
  }

  /** Full repaint (every cell), used after resize or on first frame. */
  fullRepaint(level: ColorLevel): string {
    for (let i = 0; i < this.front.length; i++) this.front[i] = blankCell();
    return this.diff(level, true);
  }
}

function sameStyle(c: Cell, fg: RGB | null | undefined, bg: RGB | null | undefined, attrs: SGRAttrs | undefined): boolean {
  if (attrs === undefined) return false;
  if (!rgbEq(c.fg, fg ?? null)) return false;
  if (!rgbEq(c.bg, bg ?? null)) return false;
  return attrsEq(c.attrs, attrs);
}

export { CURSOR_HIDE };
