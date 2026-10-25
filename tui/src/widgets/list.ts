// A virtualized, scrollable, selectable list: only visible rows are
// drawn or registered as zones, so a 10,000-item list costs the same
// as a 10-item one.
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import type { RGB } from "../term/color.js";
import type { ZoneRegistry } from "./zones.js";
import { drawLine } from "./text.js";
import { clampScroll, visibleRange, type RowHeights, type ScrollState } from "../layout/scroll.js";

export interface ListItem {
  id: string;
  label: string;
  data?: unknown;
}

export interface ListStyle {
  fg?: RGB | null;
  bg?: RGB | null;
  selectedFg?: RGB | null;
  selectedBg?: RGB | null;
}

export function drawList(
  screen: Screen,
  zones: ZoneRegistry,
  idPrefix: string,
  rect: Rect,
  items: ListItem[],
  selectedIndex: number,
  scroll: ScrollState,
  style: ListStyle = {}
): ScrollState {
  const rows: RowHeights = { count: items.length, heightOf: () => 1 };
  const clamped = clampScroll(scroll, rows, rect.height);
  const visible = visibleRange(clamped, rows, rect.height);
  for (const v of visible) {
    const item = items[v.index]!;
    const selected = v.index === selectedIndex;
    const row = rect.row + v.screenRow;
    const lineStyle = selected ? { fg: style.selectedFg ?? null, bg: style.selectedBg ?? null } : { fg: style.fg ?? null, bg: style.bg ?? null };
    if (selected) screen.fillRect(row, rect.col, rect.width, 1, " ", { bg: style.selectedBg ?? null });
    drawLine(screen, rect, row, item.label, lineStyle, true);
    zones.register(`${idPrefix}:${item.id}`, { row, col: rect.col, width: rect.width, height: 1 }, "list-item", { index: v.index, item });
  }
  return clamped;
}
