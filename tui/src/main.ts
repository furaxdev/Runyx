#!/usr/bin/env node
// Entry point: boots the terminal, makes sure a daemon is reachable
// (starting one if needed, see src/api/daemon.ts), picks a model to
// chat with, and runs the Chat screen until the user quits.
import { writeFileSync } from "node:fs";
import { Terminal, screenToSVG } from "./term/terminal.js";
import { Screen } from "./term/buffer.js";
import { DARK_THEME } from "./term/color.js";
import { ensureDaemon, DaemonUnavailableError } from "./api/daemon.js";
import { RunyxClient } from "./api/client.js";
import { runApp, drawPalette } from "./app.js";

async function pickModel(client: RunyxClient): Promise<string | null> {
  const models = await client.listModels().catch(() => []);
  return models[0]?.name ?? null;
}

/**
 * Renders one static, network-free frame to an SVG file and exits --
 * for visual regression / CI snapshotting without a live terminal or
 * daemon. Currently covers only the Ctrl+K palette (the one screen
 * whose rendering needs no async data): every other screen's draw
 * function is entangled with its own network-fetching event loop and
 * isn't dump-frame-able yet (see TUI_REPORT.md).
 */
function dumpFrame(outPath: string): void {
  const screen = new Screen(80, 24);
  screen.clear();
  drawPalette(screen, DARK_THEME, "", 0);
  screen.fullRepaint("truecolor"); // syncs the back buffer into front, which screenToSVG reads
  const svg = screenToSVG(screen, DARK_THEME);
  writeFileSync(outPath, svg, "utf8");
}

async function main(): Promise<void> {
  const dumpIdx = process.argv.indexOf("--dump-frame");
  if (dumpIdx !== -1) {
    const outPath = process.argv[dumpIdx + 1];
    if (!outPath) {
      process.stderr.write("runyx-tui: --dump-frame requires an output path\n");
      process.exitCode = 1;
      return;
    }
    dumpFrame(outPath);
    return;
  }
  let baseUrl: string;
  try {
    baseUrl = await ensureDaemon();
  } catch (e) {
    if (e instanceof DaemonUnavailableError) {
      process.stderr.write(`runyx-tui: ${e.message}\n`);
      process.exitCode = 1;
      return;
    }
    throw e;
  }

  const client = new RunyxClient(baseUrl);
  const model = await pickModel(client);
  if (!model) {
    process.stderr.write("runyx-tui: no model installed yet. Run `runyx pull <model>` first.\n");
    process.exitCode = 1;
    return;
  }

  const terminal = new Terminal();
  await terminal.start();
  try {
    await runApp(terminal, client, { model });
  } finally {
    terminal.stop();
  }
}

main().catch((err) => {
  process.stderr.write(`runyx-tui: fatal: ${err instanceof Error ? err.stack ?? err.message : String(err)}\n`);
  process.exitCode = 1;
});
