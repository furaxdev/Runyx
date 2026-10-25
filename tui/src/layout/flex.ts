// A small CSS-flexbox-like single-axis layout: given a rectangle and a
// direction, divides it among children with fixed, "auto" (content-
// sized, must be precomputed by the caller) or flexible (proportional
// remaining-space) sizing, with gap and padding. Not a general
// constraint solver — one axis at a time, which is all a terminal UI
// this shaped actually needs.

export interface Rect {
  row: number;
  col: number;
  width: number;
  height: number;
}

export type FlexSize = number | { flex: number };

export interface FlexItem<T> {
  size: FlexSize;
  data: T;
}

/** Splits `total` cells among items: fixed sizes first, then flex items share the remainder by weight. */
export function distribute<T>(total: number, gap: number, items: FlexItem<T>[]): { rect: { start: number; size: number }; data: T }[] {
  const gapTotal = gap * Math.max(0, items.length - 1);
  let fixedTotal = 0;
  let flexWeight = 0;
  for (const it of items) {
    if (typeof it.size === "number") fixedTotal += it.size;
    else flexWeight += it.size.flex;
  }
  const remaining = Math.max(0, total - gapTotal - fixedTotal);
  const out: { rect: { start: number; size: number }; data: T }[] = [];
  let cursor = 0;
  let flexUsed = 0;
  for (let i = 0; i < items.length; i++) {
    const it = items[i]!;
    let size: number;
    if (typeof it.size === "number") {
      size = it.size;
    } else if (flexWeight > 0) {
      // Give the last flex item any rounding remainder so sizes sum exactly.
      const isLastFlex = !items.slice(i + 1).some((x) => typeof x.size !== "number");
      size = isLastFlex ? remaining - flexUsed : Math.floor((remaining * it.size.flex) / flexWeight);
      flexUsed += size;
    } else {
      size = 0;
    }
    out.push({ rect: { start: cursor, size: Math.max(0, size) }, data: it.data });
    cursor += size + gap;
  }
  return out;
}

export interface FlexOptions {
  gap?: number;
  padding?: number;
}

/** Lays out items along `direction` inside `outer`, returning each item's absolute Rect. */
export function layoutFlex<T>(outer: Rect, direction: "row" | "column", items: FlexItem<T>[], opts: FlexOptions = {}): { rect: Rect; data: T }[] {
  const pad = opts.padding ?? 0;
  const gap = opts.gap ?? 0;
  const inner: Rect = {
    row: outer.row + pad,
    col: outer.col + pad,
    width: Math.max(0, outer.width - pad * 2),
    height: Math.max(0, outer.height - pad * 2),
  };
  const mainTotal = direction === "row" ? inner.width : inner.height;
  const distributed = distribute(mainTotal, gap, items);
  return distributed.map(({ rect, data }) => ({
    data,
    rect:
      direction === "row"
        ? { row: inner.row, col: inner.col + rect.start, width: rect.size, height: inner.height }
        : { row: inner.row + rect.start, col: inner.col, width: inner.width, height: rect.size },
  }));
}

export function insetRect(r: Rect, amount: number): Rect {
  return {
    row: r.row + amount,
    col: r.col + amount,
    width: Math.max(0, r.width - amount * 2),
    height: Math.max(0, r.height - amount * 2),
  };
}
