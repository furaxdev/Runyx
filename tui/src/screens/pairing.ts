// Pairing screen: issues a fresh one-time device token and shows the
// resulting QR code as unicode half-blocks (two module rows per
// terminal row, same "2 pixels per character" trick the mascot's
// sprite renderer uses), from the server's raw module grid
// (POST /api/runyx/pairing/qrcode-matrix) -- no QR-encoding logic
// lives in the TUI; internal/pairing (already using a real, tested Go
// QR library) is the only encoder, same as the PNG the web UI shows.
import type { Terminal } from "../term/terminal.js";
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import { insetRect } from "../layout/flex.js";
import { drawBox } from "../widgets/box.js";
import { drawLine, drawWrappedText } from "../widgets/text.js";
import type { RunyxClient } from "../api/client.js";
import type { RGB } from "../term/color.js";

export interface PairingScreenResult {
  action: "back";
}

/** Draws modules[r][c] (true = dark) as half-block characters, 2 module-rows per terminal row. */
export function drawQRCode(screen: Screen, rect: Rect, modules: boolean[][], dark: RGB, light: RGB): void {
  const rows = modules.length;
  const cols = modules[0]?.length ?? 0;
  for (let r = 0; r < rows; r += 2) {
    const screenRow = rect.row + r / 2;
    if (screenRow >= rect.row + rect.height) break;
    for (let c = 0; c < cols; c++) {
      if (rect.col + c >= rect.col + rect.width) break;
      const top = modules[r]?.[c] ?? false;
      const bottom = modules[r + 1]?.[c] ?? false;
      let ch = " ";
      if (top && bottom) ch = "█"; // full block
      else if (top) ch = "▀"; // upper half
      else if (bottom) ch = "▄"; // lower half
      screen.setCluster(screenRow, rect.col + c, ch, 1, { fg: dark, bg: light });
    }
  }
}

export async function runPairingScreen(terminal: Terminal, client: RunyxClient): Promise<PairingScreenResult> {
  let modules: boolean[][] | null = null;
  let url = "";
  let error: string | null = null;

  async function load(): Promise<void> {
    try {
      const qr = await client.pairingQRCode("tui-device");
      modules = qr.modules;
      url = qr.url;
      error = null;
    } catch (e) {
      error = (e as Error).message ?? String(e);
    }
    terminal.requestRender();
  }

  function draw(screen: Screen): void {
    const theme = terminal.theme;
    const full: Rect = { row: 0, col: 0, width: screen.width, height: screen.height };
    const inner = drawBox(screen, full, { title: "Pair a device", borderFg: theme.border, fg: theme.accent });

    if (error) {
      drawWrappedText(screen, insetRect(inner, 0), `error: ${error}\n\nr: retry  Esc/q: back`, { fg: theme.danger });
      return;
    }
    if (!modules) {
      drawLine(screen, inner, inner.row, "requesting pairing code...", { fg: theme.textMuted });
      return;
    }

    const cols = modules[0]?.length ?? 0;
    const qrRect: Rect = { row: inner.row, col: inner.col, width: cols, height: Math.ceil(modules.length / 2) };
    drawQRCode(screen, qrRect, modules, theme.text, theme.background);

    const footerRow = qrRect.row + qrRect.height + 1;
    const footerRect: Rect = { row: footerRow, col: inner.col, width: inner.width, height: Math.max(0, inner.height - (footerRow - inner.row)) };
    drawWrappedText(
      screen,
      footerRect,
      `Scan with the Runyx Android app, or open on the paired device:\n${url}\n\nr: new code  Esc/q: back`,
      { fg: theme.textMuted }
    );
  }

  terminal.setDrawFn(draw);
  await load();

  return new Promise<PairingScreenResult>((resolve) => {
    const onKey = (e: { key: string; ctrl: boolean }) => {
      if (e.key === "Escape" || e.key === "q") {
        terminal.off("key", onKey);
        resolve({ action: "back" });
        return;
      }
      if (e.key === "r") void load();
      terminal.requestRender();
    };
    terminal.on("key", onKey);
    terminal.requestRender();
  });
}
