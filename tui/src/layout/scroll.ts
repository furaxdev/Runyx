// Virtualized vertical scrolling: tracks a scroll offset into a list of
// `itemCount` rows (each may have its own height, e.g. wrapped message
// lines) without ever materializing off-screen rows, so a 10,000+ line
// chat transcript costs the same per frame as a short one.

export interface RowHeights {
  /** Total number of logical rows. */
  count: number;
  /** Height in terminal rows of logical row i (most content is 1, wrapped text may be more). */
  heightOf(i: number): number;
}

export interface ScrollState {
  /** First logical row index currently scrolled to the top of the viewport. */
  topIndex: number;
  /** How many of topIndex's own rows are scrolled past (for mid-row scroll). */
  topSkip: number;
}

export function clampScroll(state: ScrollState, rows: RowHeights, viewportHeight: number): ScrollState {
  if (rows.count === 0) return { topIndex: 0, topSkip: 0 };
  let total = 0;
  for (let i = 0; i < rows.count; i++) total += rows.heightOf(i);
  if (total <= viewportHeight) return { topIndex: 0, topSkip: 0 };

  let idx = Math.min(Math.max(0, state.topIndex), rows.count - 1);
  let skip = Math.max(0, state.topSkip);
  const h = rows.heightOf(idx);
  if (skip >= h) {
    skip = h - 1;
  }
  // Ensure we don't scroll past the point where the last row is fully visible.
  let fromEnd = 0;
  let lastIdx = rows.count - 1;
  while (lastIdx >= 0 && fromEnd < viewportHeight) {
    fromEnd += rows.heightOf(lastIdx);
    lastIdx--;
  }
  const maxTopIndex = lastIdx + 1;
  if (idx > maxTopIndex) {
    idx = maxTopIndex;
    skip = 0;
  }
  return { topIndex: idx, topSkip: skip };
}

/** Visible logical rows (with their on-screen row offset) for the current scroll state. */
export function visibleRange(state: ScrollState, rows: RowHeights, viewportHeight: number): { index: number; screenRow: number; rowHeight: number }[] {
  const out: { index: number; screenRow: number; rowHeight: number }[] = [];
  let screenRow = state.topSkip === 0 ? 0 : -state.topSkip;
  let i = state.topIndex;
  while (i < rows.count && screenRow < viewportHeight) {
    const h = rows.heightOf(i);
    out.push({ index: i, screenRow, rowHeight: h });
    screenRow += h;
    i++;
  }
  return out;
}

export function scrollBy(state: ScrollState, rows: RowHeights, viewportHeight: number, deltaRows: number): ScrollState {
  if (rows.count === 0) return state;
  let idx = state.topIndex;
  let skip = state.topSkip + deltaRows;
  while (skip < 0 && idx > 0) {
    idx--;
    skip += rows.heightOf(idx);
  }
  while (idx < rows.count && skip >= rows.heightOf(idx)) {
    skip -= rows.heightOf(idx);
    idx++;
  }
  if (idx >= rows.count) {
    idx = rows.count - 1;
    skip = 0;
  }
  return clampScroll({ topIndex: Math.max(0, idx), topSkip: Math.max(0, skip) }, rows, viewportHeight);
}

export function scrollToEnd(rows: RowHeights, viewportHeight: number): ScrollState {
  return clampScroll({ topIndex: rows.count, topSkip: 0 }, rows, viewportHeight);
}

/** During a drag near the top/bottom edge of a viewport, returns the autoscroll delta (rows/tick) to apply, or 0. */
export function autoscrollDelta(pointerRow: number, viewportTop: number, viewportHeight: number, edgeZone = 2): number {
  const rel = pointerRow - viewportTop;
  if (rel < edgeZone) return -(edgeZone - rel);
  if (rel > viewportHeight - edgeZone) return rel - (viewportHeight - edgeZone);
  return 0;
}
