// App shell: cycles between screens and owns the Ctrl+K command
// palette. Each screen runs its own event loop (see src/screens/*.ts)
// and resolves back here with {action:"back"} when the user leaves it;
// the palette is the only way back in from a screen other than its own
// Esc/q, since screens don't know about each other.
import type { Terminal } from "./term/terminal.js";
import type { Screen } from "./term/buffer.js";
import type { Rect } from "./layout/flex.js";
import type { Theme } from "./term/color.js";
import { drawBox } from "./widgets/box.js";
import { drawLine } from "./widgets/text.js";
import type { RunyxClient } from "./api/client.js";
import { runChatScreen } from "./screens/chat.js";
import { runDashboardScreen } from "./screens/dashboard.js";
import { runModelsScreen } from "./screens/models.js";
import { runPermissionsScreen } from "./screens/permissions.js";
import { runApprovalsScreen } from "./screens/approvals.js";
import { runPairingScreen } from "./screens/pairing.js";

type ScreenName = "chat" | "dashboard" | "models" | "permissions" | "approvals" | "pairing";

interface PaletteCommand {
  id: ScreenName;
  label: string;
}

export const COMMANDS: PaletteCommand[] = [
  { id: "chat", label: "Chat" },
  { id: "dashboard", label: "Dashboard" },
  { id: "models", label: "Models" },
  { id: "permissions", label: "Permissions" },
  { id: "approvals", label: "Approvals" },
  { id: "pairing", label: "Pair a device" },
];

function filterCommands(query: string): PaletteCommand[] {
  const q = query.toLowerCase();
  return COMMANDS.filter((c) => c.label.toLowerCase().includes(q));
}

/**
 * Draws the Ctrl+K palette overlay for a given query/selection,
 * independent of any live Terminal/event loop -- exported so
 * `--dump-frame` (see main.ts) can render a representative static
 * frame of the TUI without a running daemon, the same screen-rendering
 * code the interactive palette below actually uses.
 */
export function drawPalette(screen: Screen, theme: Theme, query: string, selected: number): void {
  const width = Math.min(50, screen.width - 4);
  const height = Math.min(10, screen.height - 4);
  const rect: Rect = {
    row: Math.floor((screen.height - height) / 2),
    col: Math.floor((screen.width - width) / 2),
    width,
    height,
  };
  const inner = drawBox(screen, rect, { title: "Go to...", borderFg: theme.accent, fg: theme.accent, bg: theme.surfaceElevated });
  drawLine(screen, inner, inner.row, `> ${query}`, { fg: theme.text, bg: theme.surfaceElevated });
  const items = filterCommands(query);
  items.forEach((cmd, i) => {
    const row = inner.row + 2 + i;
    if (row >= inner.row + inner.height) return;
    drawLine(screen, inner, row, cmd.label, {
      fg: i === selected ? theme.accentContrast : theme.text,
      bg: i === selected ? theme.accent : theme.surfaceElevated,
    });
  });
}

/**
 * Shows the Ctrl+K palette and waits for the user to pick a screen, or
 * cancel. Draws directly (bypassing the caller's draw function) and
 * restores it on exit.
 */
async function runPalette(terminal: Terminal): Promise<ScreenName | null> {
  let query = "";
  let selected = 0;

  function filtered(): PaletteCommand[] {
    return filterCommands(query);
  }

  function draw(screen: Screen): void {
    drawPalette(screen, terminal.theme, query, selected);
  }

  terminal.setDrawFn(draw);
  terminal.requestRender();

  return new Promise<ScreenName | null>((resolve) => {
    const onKey = (e: { key: string; ctrl: boolean }) => {
      const items = filtered();
      if (e.key === "Escape") {
        terminal.off("key", onKey);
        resolve(null);
        return;
      }
      if (e.key === "Enter") {
        terminal.off("key", onKey);
        resolve(items[selected]?.id ?? null);
        return;
      }
      if (e.key === "Up") selected = Math.max(0, selected - 1);
      else if (e.key === "Down") selected = Math.min(Math.max(0, items.length - 1), selected + 1);
      else if (e.key === "Backspace") query = query.slice(0, -1);
      else if (e.key.length === 1 && !e.ctrl) {
        query += e.key;
        selected = 0;
      }
      terminal.requestRender();
    };
    terminal.on("key", onKey);
  });
}

export interface AppOptions {
  model: string;
}

/**
 * Runs the Chat screen by default; Ctrl+K from within any screen's own
 * idle moment isn't intercepted here (each screen owns its key loop
 * while active) -- instead, a screen exits back to this loop on its
 * own Esc/q, and this loop immediately shows the palette so the user
 * can jump elsewhere. Ctrl+C always quits outright.
 */
export async function runApp(terminal: Terminal, client: RunyxClient, opts: AppOptions): Promise<void> {
  let current: ScreenName = "chat";
  let quitting = false;

  const onGlobalKey = (e: { key: string; ctrl: boolean }) => {
    if (e.ctrl && e.key === "c") quitting = true;
  };
  terminal.on("key", onGlobalKey);

  while (!quitting) {
    if (current === "chat") {
      await runChatScreen(terminal, client, { model: opts.model });
      if (quitting) break;
    } else if (current === "dashboard") {
      await runDashboardScreen(terminal, client);
    } else if (current === "models") {
      await runModelsScreen(terminal, client);
    } else if (current === "permissions") {
      await runPermissionsScreen(terminal, client);
    } else if (current === "approvals") {
      await runApprovalsScreen(terminal, client);
    } else if (current === "pairing") {
      await runPairingScreen(terminal, client);
    }
    if (quitting) break;
    const next = await runPalette(terminal);
    if (next) current = next;
    else current = "chat";
  }

  terminal.off("key", onGlobalKey);
}
