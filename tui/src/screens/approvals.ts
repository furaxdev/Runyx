// Approvals screen: the human side of internal/approval -- lists every
// pending request (shell commands today), shows the full copyable
// detail, and lets the user approve or deny. Polls every second;
// there is no push channel for this yet (SSE for approvals is a
// natural follow-up once Remote Control needs to notify a phone).
import type { Terminal } from "../term/terminal.js";
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import { drawBox } from "../widgets/box.js";
import { drawLine } from "../widgets/text.js";
import { drawWrappedText } from "../widgets/text.js";
import type { RunyxClient } from "../api/client.js";
import type { ApprovalRequest } from "../api/types.js";

const POLL_MS = 1000;

export interface ApprovalsScreenResult {
  action: "back";
}

export async function runApprovalsScreen(terminal: Terminal, client: RunyxClient): Promise<ApprovalsScreenResult> {
  let pending: ApprovalRequest[] = [];
  let error: string | null = null;
  let selected = 0;
  let status = "";
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  async function refresh(): Promise<void> {
    try {
      pending = await client.listApprovals();
      error = null;
      if (selected >= pending.length) selected = Math.max(0, pending.length - 1);
    } catch (e) {
      error = (e as Error).message ?? String(e);
    }
    terminal.requestRender();
  }

  function draw(screen: Screen): void {
    const theme = terminal.theme;
    const full: Rect = { row: 0, col: 0, width: screen.width, height: screen.height };
    const inner = drawBox(screen, full, { title: "Pending approvals", borderFg: theme.border, fg: theme.accent });
    const [listRect, detailRect, footerRect] = splitThree(inner);

    if (error) {
      drawLine(screen, listRect, listRect.row, `error: ${error}`, { fg: theme.danger });
      return;
    }
    if (pending.length === 0) {
      drawLine(screen, listRect, listRect.row, "No pending approvals.", { fg: theme.textMuted });
    }
    pending.forEach((req, i) => {
      const row = listRect.row + i;
      if (row >= listRect.row + listRect.height) return;
      const marker = i === selected ? ">" : " ";
      drawLine(screen, listRect, row, `${marker} [${req.kind}] ${req.summary}`, {
        fg: theme.text,
        bg: i === selected ? theme.surfaceElevated : null,
      }, true);
    });

    const current = pending[selected];
    if (current) {
      drawLine(screen, detailRect, detailRect.row, "Detail:", { fg: theme.text, attrs: { bold: true } });
      drawWrappedText(screen, { ...detailRect, row: detailRect.row + 1, height: detailRect.height - 1 }, current.detail, {
        fg: theme.textMuted,
      });
    }

    drawLine(screen, footerRect, footerRect.row, status || "Up/Down: select  a: approve  d: deny  Esc/q: back", {
      fg: status ? theme.warning : theme.textMuted,
    });
  }

  terminal.setDrawFn(draw);
  await refresh();
  pollTimer = setInterval(() => void refresh(), POLL_MS);

  return new Promise<ApprovalsScreenResult>((resolve) => {
    const onKey = (e: { key: string; ctrl: boolean }) => {
      if (e.key === "Escape" || e.key === "q") {
        if (pollTimer) clearInterval(pollTimer);
        terminal.off("key", onKey);
        resolve({ action: "back" });
        return;
      }
      const current = pending[selected];
      if (e.key === "Up") selected = Math.max(0, selected - 1);
      else if (e.key === "Down") selected = Math.min(Math.max(0, pending.length - 1), selected + 1);
      else if ((e.key === "a" || e.key === "d") && current) {
        const allow = e.key === "a";
        void client
          .decideApproval(current.id, current.hash, allow)
          .then(() => {
            status = allow ? `approved: ${current.summary}` : `denied: ${current.summary}`;
            return refresh();
          })
          .catch((err) => {
            status = `decision failed: ${(err as Error).message ?? err}`;
            terminal.requestRender();
          });
      }
      terminal.requestRender();
    };
    terminal.on("key", onKey);
    terminal.requestRender();
  });
}

function splitThree(inner: Rect): [Rect, Rect, Rect] {
  const listHeight = Math.max(1, Math.floor(inner.height * 0.4));
  const footerHeight = 1;
  const detailHeight = Math.max(0, inner.height - listHeight - footerHeight - 1);
  return [
    { row: inner.row, col: inner.col, width: inner.width, height: listHeight },
    { row: inner.row + listHeight + 1, col: inner.col, width: inner.width, height: detailHeight },
    { row: inner.row + inner.height - footerHeight, col: inner.col, width: inner.width, height: footerHeight },
  ];
}
