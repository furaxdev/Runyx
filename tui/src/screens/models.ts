// Models screen: a scrollable view of the curated registry
// (/api/runyx/registry) merged with install state and hardware fit --
// the same data the web UI's Models page renders as cards. "p" on a
// not-yet-installed model streams its download (NDJSON progress from
// /api/runyx/models/pull) with a real progress bar; Ctrl+X cancels
// (the server's downloader leaves a resumable .part file, so a later
// pull of the same model picks the saved state back up).
import type { Terminal } from "../term/terminal.js";
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import { insetRect } from "../layout/flex.js";
import { drawBox } from "../widgets/box.js";
import { drawLine } from "../widgets/text.js";
import { drawProgressBar } from "../widgets/progressbar.js";
import { clampScroll, visibleRange, type RowHeights } from "../layout/scroll.js";
import type { RunyxClient } from "../api/client.js";
import type { RegistryEntry } from "../api/types.js";

export interface ModelsScreenResult {
  action: "back";
}

function fitLabel(fit: RegistryEntry["fit"]): string {
  switch (fit) {
    case "comfortable":
      return "comfortable";
    case "ok":
      return "ok";
    case "tight":
      return "tight";
    case "too_large":
      return "too large";
    default:
      return fit;
  }
}

export async function runModelsScreen(terminal: Terminal, client: RunyxClient): Promise<ModelsScreenResult> {
  let entries: RegistryEntry[] = [];
  let error: string | null = null;
  let scroll = { topIndex: 0, topSkip: 0 };
  let selected = 0;

  let pulling = false;
  let pullTarget = "";
  let pullDownloaded = 0;
  let pullTotal = 0;
  let pullError = "";
  let abortController: AbortController | null = null;

  async function load(): Promise<void> {
    try {
      entries = await client.listRegistry();
      error = null;
    } catch (e) {
      error = (e as Error).message ?? String(e);
    }
    terminal.requestRender();
  }

  async function pull(entry: RegistryEntry): Promise<void> {
    if (pulling) return;
    pulling = true;
    pullTarget = entry.name;
    pullDownloaded = 0;
    pullTotal = 0;
    pullError = "";
    abortController = new AbortController();
    try {
      await client.pullModel(
        entry.id,
        (p) => {
          pullDownloaded = p.downloaded;
          pullTotal = p.total;
          terminal.requestRender();
        },
        abortController.signal
      );
      await load();
    } catch (e) {
      if (!abortController.signal.aborted) {
        pullError = (e as Error).message ?? String(e);
      }
    } finally {
      pulling = false;
      abortController = null;
      terminal.requestRender();
    }
  }

  function draw(screen: Screen): void {
    const theme = terminal.theme;
    const full: Rect = { row: 0, col: 0, width: screen.width, height: screen.height };
    const inner = drawBox(screen, full, { title: "Models", borderFg: theme.border, fg: theme.accent });
    const pullBarHeight = pulling || pullError ? 2 : 0;
    const body: Rect = { ...insetRect(inner, 0), height: Math.max(0, inner.height - 1 - pullBarHeight) };
    const pullRect: Rect = { row: body.row + body.height, col: body.col, width: body.width, height: pullBarHeight };
    const footer: Rect = { row: inner.row + inner.height - 1, col: inner.col, width: inner.width, height: 1 };

    if (error) {
      drawLine(screen, body, body.row, `error: ${error}`, { fg: theme.danger });
      return;
    }

    const rows: RowHeights = { count: entries.length, heightOf: () => 1 };
    if (selected < scroll.topIndex) scroll = { topIndex: selected, topSkip: 0 };
    else if (selected >= scroll.topIndex + body.height) scroll = { topIndex: selected - body.height + 1, topSkip: 0 };
    scroll = clampScroll(scroll, rows, body.height);
    const visible = visibleRange(scroll, rows, body.height);
    for (const v of visible) {
      const m = entries[v.index]!;
      const sizeGB = (m.size_bytes / 1e9).toFixed(1);
      const flag = m.installed ? "[installed]" : "";
      const marker = v.index === selected ? ">" : " ";
      const line = `${marker} ${m.name.padEnd(28)} ${sizeGB.padStart(6)}GB  ${fitLabel(m.fit).padEnd(11)} ${flag}`;
      drawLine(
        screen,
        body,
        body.row + v.screenRow,
        line,
        { fg: m.installed ? theme.success : theme.text, bg: v.index === selected ? theme.surfaceElevated : null },
        true
      );
    }

    if (pulling) {
      const fraction = pullTotal > 0 ? pullDownloaded / pullTotal : 0;
      const mb = (n: number) => (n / 1e6).toFixed(1);
      drawLine(screen, pullRect, pullRect.row, `Pulling ${pullTarget}: ${mb(pullDownloaded)}MB / ${mb(pullTotal)}MB`, {
        fg: theme.text,
      });
      drawProgressBar(screen, { ...pullRect, row: pullRect.row + 1, height: 1 }, fraction, {
        fg: theme.accent,
        trackBg: theme.surfaceElevated,
      });
    } else if (pullError) {
      drawLine(screen, pullRect, pullRect.row, `pull failed: ${pullError}`, { fg: theme.danger });
    }

    drawLine(screen, footer, footer.row, "Esc/q: back  Up/Down: select  p: pull  Ctrl+X: cancel pull  r: refresh", {
      fg: theme.textMuted,
    });
  }

  terminal.setDrawFn(draw);
  await load();

  return new Promise<ModelsScreenResult>((resolve) => {
    const onKey = (e: { key: string; ctrl: boolean }) => {
      if (e.key === "Escape" || e.key === "q") {
        terminal.off("key", onKey);
        resolve({ action: "back" });
        return;
      }
      if (e.ctrl && e.key === "x") {
        abortController?.abort();
        return;
      }
      if (e.key === "r") void load();
      else if (e.key === "Up") selected = Math.max(0, selected - 1);
      else if (e.key === "Down") selected = Math.min(Math.max(0, entries.length - 1), selected + 1);
      else if (e.key === "p" && entries[selected] && !entries[selected]!.installed) {
        void pull(entries[selected]!);
      }
      terminal.requestRender();
    };
    terminal.on("key", onKey);
    terminal.requestRender();
  });
}
