// A clickable label: draws text, registers a zone for mouse hit-testing
// and keyboard-focus highlighting.
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import type { RGB } from "../term/color.js";
import type { ZoneRegistry } from "./zones.js";

export interface ButtonStyle {
  fg?: RGB | null;
  bg?: RGB | null;
  focused?: boolean;
  focusFg?: RGB | null;
  focusBg?: RGB | null;
}

export function drawButton(screen: Screen, zones: ZoneRegistry, id: string, rect: Rect, label: string, data: unknown, style: ButtonStyle = {}): void {
  const fg = style.focused ? style.focusFg ?? style.fg ?? null : style.fg ?? null;
  const bg = style.focused ? style.focusBg ?? style.bg ?? null : style.bg ?? null;
  screen.fillRect(rect.row, rect.col, rect.width, rect.height, " ", { bg });
  const pad = Math.max(0, Math.floor((rect.width - stringDisplayWidth(label)) / 2));
  screen.writeText(rect.row, rect.col + pad, label, { fg, bg, attrs: style.focused ? { bold: true } : {} });
  zones.register(id, rect, "button", data);
}

function stringDisplayWidth(s: string): number {
  // Cheap width estimate for centering only (exact grapheme width isn't
  // worth it here); good enough for ASCII button labels.
  return s.length;
}
