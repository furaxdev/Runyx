// A progress bar using eighth-block characters for sub-cell precision
// (so progress moves smoothly even at narrow widths), ASCII fallback.
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import type { RGB } from "../term/color.js";

const BLOCKS = ["", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"];

export function drawProgressBar(
  screen: Screen,
  rect: Rect,
  fraction: number,
  style: { fg?: RGB | null; bg?: RGB | null; trackBg?: RGB | null; ascii?: boolean } = {}
): void {
  const row = rect.row;
  const width = rect.width;
  const f = Math.max(0, Math.min(1, fraction));
  if (style.ascii) {
    const filled = Math.round(f * width);
    const bar = "#".repeat(filled) + "-".repeat(Math.max(0, width - filled));
    screen.writeText(row, rect.col, bar, { fg: style.fg ?? null, bg: style.bg ?? null });
    return;
  }
  screen.fillRect(row, rect.col, width, 1, " ", { bg: style.trackBg ?? null });
  const totalEighths = Math.round(f * width * 8);
  const fullCells = Math.floor(totalEighths / 8);
  const remainder = totalEighths % 8;
  for (let i = 0; i < fullCells && i < width; i++) {
    screen.setCluster(row, rect.col + i, "█", 1, { fg: style.fg ?? null, bg: style.bg ?? null });
  }
  if (remainder > 0 && fullCells < width) {
    screen.setCluster(row, rect.col + fullCells, BLOCKS[remainder]!, 1, { fg: style.fg ?? null, bg: style.trackBg ?? null });
  }
}
