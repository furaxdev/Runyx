// Dashboard screen: a read-only view of /api/runyx/dashboard -- which
// models are currently loaded (port, idle time), total RAM/VRAM, and
// the active backend. Refreshes on an interval since nothing pushes
// this data to the client.
import type { Terminal } from "../term/terminal.js";
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import { insetRect } from "../layout/flex.js";
import { drawBox } from "../widgets/box.js";
import { drawLine } from "../widgets/text.js";
import type { RunyxClient } from "../api/client.js";
import type { DashboardStats } from "../api/types.js";

const REFRESH_MS = 3000;

export interface DashboardScreenResult {
  action: "back";
}

export async function runDashboardScreen(terminal: Terminal, client: RunyxClient): Promise<DashboardScreenResult> {
  let stats: DashboardStats | null = null;
  let error: string | null = null;
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  async function refresh(): Promise<void> {
    try {
      stats = await client.dashboardStats();
      error = null;
    } catch (e) {
      error = (e as Error).message ?? String(e);
    }
    terminal.requestRender();
  }

  function draw(screen: Screen): void {
    const theme = terminal.theme;
    const full: Rect = { row: 0, col: 0, width: screen.width, height: screen.height };
    const inner = drawBox(screen, full, { title: "Dashboard", borderFg: theme.border, fg: theme.accent });
    const body = insetRect(inner, 0);

    if (error) {
      drawLine(screen, body, body.row, `error: ${error}`, { fg: theme.danger });
      return;
    }
    if (!stats) {
      drawLine(screen, body, body.row, "loading...", { fg: theme.textMuted });
      return;
    }

    let row = body.row;
    drawLine(screen, body, row++, `Backend: ${stats.backend}`, { fg: theme.text });
    drawLine(screen, body, row++, `Total RAM: ${stats.total_ram_mb} MB`, { fg: theme.text });
    drawLine(screen, body, row++, `Total VRAM: ${stats.total_vram_mb} MB`, { fg: theme.text });
    row++;
    drawLine(screen, body, row++, "Loaded models:", { fg: theme.text, attrs: { bold: true } });
    if (stats.loaded_models.length === 0) {
      drawLine(screen, body, row++, "  (none)", { fg: theme.textMuted });
    }
    for (const m of stats.loaded_models) {
      if (row >= body.row + body.height) break;
      drawLine(screen, body, row++, `  ${m.name}  port=${m.port}  idle=${m.idle_seconds}s`, { fg: theme.text });
    }
    row++;
    drawLine(screen, body, row, "Esc/q: back  r: refresh now", { fg: theme.textMuted });
  }

  terminal.setDrawFn(draw);
  await refresh();
  refreshTimer = setInterval(() => void refresh(), REFRESH_MS);

  return new Promise<DashboardScreenResult>((resolve) => {
    const onKey = (e: { key: string; ctrl: boolean }) => {
      if (e.key === "Escape" || e.key === "q") {
        if (refreshTimer) clearInterval(refreshTimer);
        terminal.off("key", onKey);
        resolve({ action: "back" });
        return;
      }
      if (e.key === "r") void refresh();
    };
    terminal.on("key", onKey);
    terminal.requestRender();
  });
}
