// Permissions screen: a read-only view of /api/runyx/permissions (shell
// agent scope, vision/voice/web-search flags, paired devices). Device
// revocation is wired up (Enter on a device sends the DELETE); nothing
// else here is mutable yet.
import type { Terminal } from "../term/terminal.js";
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import { insetRect } from "../layout/flex.js";
import { drawBox } from "../widgets/box.js";
import { drawLine } from "../widgets/text.js";
import type { RunyxClient } from "../api/client.js";
import type { PermissionsSnapshot } from "../api/types.js";

export interface PermissionsScreenResult {
  action: "back";
}

export async function runPermissionsScreen(terminal: Terminal, client: RunyxClient): Promise<PermissionsScreenResult> {
  let snap: PermissionsSnapshot | null = null;
  let error: string | null = null;
  let selected = 0;
  let status = "";

  async function load(): Promise<void> {
    try {
      snap = await client.permissions();
      error = null;
    } catch (e) {
      error = (e as Error).message ?? String(e);
    }
    terminal.requestRender();
  }

  function draw(screen: Screen): void {
    const theme = terminal.theme;
    const full: Rect = { row: 0, col: 0, width: screen.width, height: screen.height };
    const inner = drawBox(screen, full, { title: "Permissions", borderFg: theme.border, fg: theme.accent });
    const body = insetRect(inner, 0);

    if (error) {
      drawLine(screen, body, body.row, `error: ${error}`, { fg: theme.danger });
      return;
    }
    if (!snap) {
      drawLine(screen, body, body.row, "loading...", { fg: theme.textMuted });
      return;
    }

    let row = body.row;
    drawLine(screen, body, row++, `Shell agent: ${snap.shell_agent.enabled ? `enabled (${snap.shell_agent.work_dir})` : "disabled"}`, {
      fg: theme.text,
    });
    drawLine(screen, body, row++, `Vision: ${snap.vision_enabled ? "enabled" : "disabled"}`, { fg: theme.text });
    drawLine(screen, body, row++, `Voice: ${snap.voice_enabled ? "enabled" : "disabled"}`, { fg: theme.text });
    drawLine(screen, body, row++, `Web search: ${snap.web_search_on ? "on" : "off"}`, { fg: theme.text });
    row++;
    drawLine(screen, body, row++, "Paired devices (Enter to revoke):", { fg: theme.text, attrs: { bold: true } });
    if (snap.devices.length === 0) {
      drawLine(screen, body, row++, "  (none)", { fg: theme.textMuted });
    }
    snap.devices.forEach((d, i) => {
      if (row >= body.row + body.height - 1) return;
      const marker = i === selected ? ">" : " ";
      drawLine(screen, body, row++, `${marker} ${d.name || d.id}  (${d.created_at})`, {
        fg: theme.text,
        bg: i === selected ? theme.surfaceElevated : null,
      });
    });
    if (status) drawLine(screen, body, body.row + body.height - 1, status, { fg: theme.warning });
  }

  terminal.setDrawFn(draw);
  await load();

  return new Promise<PermissionsScreenResult>((resolve) => {
    const onKey = (e: { key: string; ctrl: boolean }) => {
      if (e.key === "Escape" || e.key === "q") {
        terminal.off("key", onKey);
        resolve({ action: "back" });
        return;
      }
      if (!snap) return;
      if (e.key === "r") void load();
      else if (e.key === "Up") selected = Math.max(0, selected - 1);
      else if (e.key === "Down") selected = Math.min(Math.max(0, snap.devices.length - 1), selected + 1);
      else if (e.key === "Enter" && snap.devices[selected]) {
        const device = snap.devices[selected]!;
        void client
          .revokeDevice(device.id)
          .then(() => {
            status = `revoked ${device.name || device.id}`;
            return load();
          })
          .catch((err) => {
            status = `revoke failed: ${(err as Error).message ?? err}`;
            terminal.requestRender();
          });
      }
      terminal.requestRender();
    };
    terminal.on("key", onKey);
    terminal.requestRender();
  });
}
