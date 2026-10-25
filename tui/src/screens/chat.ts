// The Chat screen: a real conversation against the real daemon, over
// the same /v1/chat/completions SSE proxy the web UI uses. Deliberately
// mirrors web/src/lib/Chat.svelte's streaming contract (see
// docs/DECISIONS.md's "streaming cursor never detached" entry): the
// request carries no max_tokens/temperature cap by default (a real
// conversation isn't bounded), and a stall watchdog surfaces "model is
// slow or stalled" + a Stop hint instead of leaving a spinner running
// forever if the stream never ends.
import type { Terminal } from "../term/terminal.js";
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import { layoutFlex, insetRect } from "../layout/flex.js";
import { drawBox } from "../widgets/box.js";
import { TextBuffer, drawTextInput } from "../widgets/textinput.js";
import { drawLine } from "../widgets/text.js";
import { ZoneRegistry } from "../widgets/zones.js";
import { clampScroll, scrollBy, scrollToEnd, visibleRange, type RowHeights } from "../layout/scroll.js";
import { parseBlocks } from "../markdown/block.js";
import { renderMarkdown, type RenderedLine } from "../markdown/render.js";
import type { RunyxClient } from "../api/client.js";
import type { Conversation } from "../api/types.js";

const STALL_TIMEOUT_MS = 20000;

interface ChatTurn {
  role: "user" | "assistant";
  content: string;
}

export interface ChatScreenOptions {
  model: string;
  /** Demo/test-only determinism hooks; absent in normal interactive use. */
  maxTokens?: number;
  temperature?: number;
}

/**
 * Runs an interactive chat loop until the user quits (Ctrl+C/Ctrl+Q).
 * Creates (or reuses, if one exists) a conversation for `model`,
 * persists every turn through the daemon's history API, and streams
 * replies token-by-token.
 */
export async function runChatScreen(terminal: Terminal, client: RunyxClient, opts: ChatScreenOptions): Promise<void> {
  const conversation = await ensureConversation(client, opts.model);
  const turns: ChatTurn[] = (await client.getMessages(conversation.ID)).map((m) => ({
    role: m.Role === "assistant" ? "assistant" : "user",
    content: m.Content,
  }));

  const input = new TextBuffer();
  const zones = new ZoneRegistry();
  let scroll = { topIndex: 0, topSkip: 0 };
  let streaming = false;
  let stalled = false;
  let liveReply = "";
  let abortController: AbortController | null = null;
  let stallTimer: ReturnType<typeof setTimeout> | null = null;
  let quit = false;

  function clearStallTimer() {
    if (stallTimer) clearTimeout(stallTimer);
    stallTimer = null;
  }
  function armStallTimer() {
    clearStallTimer();
    stallTimer = setTimeout(() => {
      stalled = true;
      terminal.requestRender();
    }, STALL_TIMEOUT_MS);
  }

  function transcriptLines(width: number): RenderedLine[] {
    const out: RenderedLine[] = [];
    for (const t of turns) {
      out.push(...renderMarkdown(parseBlocks(`**${t.role === "user" ? "You" : "Assistant"}:**`), { width, italicSupported: true }));
      out.push(...renderMarkdown(parseBlocks(t.content), { width, italicSupported: true }));
      out.push({ spans: [{ text: "" }], plainText: "", source: null });
    }
    if (streaming) {
      out.push(...renderMarkdown(parseBlocks("**Assistant:**"), { width, italicSupported: true }));
      out.push(...renderMarkdown(parseBlocks(liveReply + (stalled ? "" : " █")), { width, italicSupported: true }));
      if (stalled) {
        out.push({ spans: [{ text: "[model is slow or stalled -- Ctrl+X to stop]", dim: true }], plainText: "[model is slow or stalled -- Ctrl+X to stop]", source: null });
      }
    }
    return out;
  }

  function draw(screen: Screen): void {
    zones.reset();
    const theme = terminal.theme;
    const full: Rect = { row: 0, col: 0, width: screen.width, height: screen.height };
    const [header, body, footer] = layoutFlex(full, "column", [
      { size: 1, data: "header" },
      { size: { flex: 1 }, data: "body" },
      { size: 3, data: "footer" },
    ]).map((x) => x.rect);

    drawLine(screen, header!, header!.row, ` Runyx -- ${conversation.Title || opts.model} `, {
      fg: theme.accent,
      attrs: { bold: true },
    });

    const lines = transcriptLines(Math.max(1, body!.width - 2));
    const rows: RowHeights = { count: lines.length, heightOf: () => 1 };
    scroll = clampScroll(scroll, rows, body!.height - 2);
    const inner = insetRect(body!, 1);
    const visible = visibleRange(scroll, rows, inner.height);
    for (const v of visible) {
      const line = lines[v.index]!;
      drawLine(screen, inner, inner.row + v.screenRow, line.plainText, { fg: theme.text });
    }
    drawBox(screen, body!, { borderFg: theme.border });

    const footerInner = drawBox(screen, footer!, { borderFg: streaming ? theme.warning : theme.border });
    drawTextInput(screen, footerInner, input, streaming ? "(waiting for reply...)" : "Type a message, Enter to send...", {
      fg: theme.text,
      placeholderFg: theme.border,
      cursorFg: theme.background,
      cursorBg: theme.accent,
    });
  }

  terminal.setDrawFn(draw);

  function stop(): void {
    abortController?.abort();
  }

  async function send(text: string): Promise<void> {
    if (!text.trim() || streaming) return;
    turns.push({ role: "user", content: text });
    await client.addMessage(conversation.ID, "user", text);
    input.clear();
    scroll = scrollToEnd({ count: turns.length * 3, heightOf: () => 1 }, 1);

    streaming = true;
    stalled = false;
    liveReply = "";
    abortController = new AbortController();
    armStallTimer();
    terminal.setStreaming(true);
    terminal.requestRender();
    try {
      const history = turns.map((t) => ({ role: t.role, content: t.content }));
      await client.streamChat(
        opts.model,
        history,
        (delta) => {
          liveReply += delta;
          stalled = false;
          armStallTimer();
          terminal.requestRender();
        },
        abortController.signal,
        {
          ...(opts.maxTokens !== undefined ? { maxTokens: opts.maxTokens } : {}),
          ...(opts.temperature !== undefined ? { temperature: opts.temperature } : {}),
        }
      );
      turns.push({ role: "assistant", content: liveReply });
      await client.addMessage(conversation.ID, "assistant", liveReply);
    } catch (e) {
      const aborted = abortController?.signal.aborted;
      const content = aborted ? liveReply : `${liveReply}\n\n[error: ${(e as Error).message ?? e}]`;
      if (content.trim()) {
        turns.push({ role: "assistant", content });
        await client.addMessage(conversation.ID, "assistant", content);
      }
    } finally {
      clearStallTimer();
      streaming = false;
      stalled = false;
      liveReply = "";
      abortController = null;
      terminal.setStreaming(false);
      terminal.requestRender();
    }
  }

  return new Promise<void>((resolve) => {
    const onKey = (e: { key: string; ctrl: boolean }) => {
      if (quit) return;
      if (e.ctrl && (e.key === "c" || e.key === "q")) {
        quit = true;
        terminal.off("key", onKey);
        resolve();
        return;
      }
      if (e.ctrl && e.key === "x") {
        stop();
        return;
      }
      if (e.key === "Enter" && !streaming) {
        const text = input.value();
        void send(text);
        return;
      }
      if (e.key === "Backspace") input.backspace();
      else if (e.key === "Delete") input.deleteForward();
      else if (e.key === "Left") input.moveLeft();
      else if (e.key === "Right") input.moveRight();
      else if (e.key === "Up") scroll = scrollBy(scroll, { count: 10000, heightOf: () => 1 }, 1, -1);
      else if (e.key === "Down") scroll = scrollBy(scroll, { count: 10000, heightOf: () => 1 }, 1, 1);
      else if (e.key.length === 1 && !e.ctrl) input.insert(e.key);
      terminal.requestRender();
    };
    terminal.on("key", onKey);
    terminal.requestRender();
  });
}

async function ensureConversation(client: RunyxClient, model: string): Promise<Conversation> {
  const existing = await client.listConversations().catch(() => [] as Conversation[]);
  const match = existing.find((c) => c.Model === model);
  if (match) return match;
  return client.newConversation(`Chat with ${model}`, model);
}
