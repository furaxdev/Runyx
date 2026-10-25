<script lang="ts">
  import { onMount, afterUpdate } from "svelte";
  import { fly } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { api, type Message, type RegistryModel } from "./api";
  import Composer from "./Composer.svelte";
  import { Volume2, Copy, Check, RefreshCw, ArrowDown, Download } from "./icons";

  export let conversationId: string;
  export let conversationTitle = "";
  export let model: string;
  export let messages: Message[] = [];
  export let onMessagesChanged: () => void;
  export let installedModels: RegistryModel[] = [];
  export let onBrowseMore: () => void;
  export let reduceMotion = false;

  $: messageFlyDuration = reduceMotion ? 0 : 220;
  // Set when this Chat is mounted right after Home created the
  // conversation for a just-typed first message: send it immediately
  // instead of waiting for another Composer submit.
  export let initialText = "";
  export let initialAttachments: { file_name: string; kind: string; text: string }[] = [];

  let streaming = false;
  let stalled = false;
  let liveReply = "";
  let currentAudio: HTMLAudioElement | null = null;
  let abortController: AbortController | null = null;
  let stallTimer: ReturnType<typeof setTimeout> | null = null;
  let regenerating = false;
  let copiedId: string | null = null;
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;
  let messagesEl: HTMLDivElement;
  let showScrollToBottom = false;

  async function copyMessage(id: string, content: string) {
    try {
      await navigator.clipboard.writeText(content);
      copiedId = id;
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => (copiedId = null), 1500);
    } catch {
      /* clipboard unavailable (insecure context, permissions) -- silently ignore */
    }
  }

  function onMessagesScroll() {
    if (!messagesEl) return;
    const distanceFromBottom = messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight;
    showScrollToBottom = distanceFromBottom > 200;
  }

  function scrollToBottom(behavior: ScrollBehavior = "auto") {
    messagesEl?.scrollTo({ top: messagesEl.scrollHeight, behavior });
  }

  function onScrollToBottomClick() {
    scrollToBottom(reduceMotion ? "auto" : "smooth");
  }

  // Sticks to the bottom as new content arrives (new messages, streamed
  // tokens) as long as the user hasn't deliberately scrolled up to read
  // something earlier -- the same "stick unless scrolled away" behavior
  // most chat UIs use, so a user reading history isn't yanked back down
  // mid-read. Instant ("auto"), not smooth: this can fire once per
  // streamed token, and queuing a smooth-scroll animation that often
  // would fight itself rather than look fluid.
  afterUpdate(() => {
    if (!showScrollToBottom) scrollToBottom("auto");
  });

  function formatTimestamp(iso: string): string {
    try {
      return new Date(iso).toLocaleString();
    } catch {
      return "";
    }
  }

  let exporting = false;

  async function exportConversation() {
    exporting = true;
    try {
      const markdown = await api.exportConversation(conversationId, "markdown");
      const blob = new Blob([markdown], { type: "text/markdown;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const safeName = (conversationTitle || "conversation").replace(/[^\w\- ]+/g, "").trim() || "conversation";
      const a = document.createElement("a");
      a.href = url;
      a.download = `${safeName}.md`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      alert(`Export failed: ${err}`);
    } finally {
      exporting = false;
    }
  }

  // Demo-only determinism: screenshot.mjs passes these as query params so
  // the CI demo gets a short, capped, reproducible reply instead of
  // waiting on however long the real default generation takes. Absent in
  // normal use, so real conversations are never capped by this.
  const demoParams = new URLSearchParams(location.search);
  const demoMaxTokens = demoParams.has("demoMaxTokens") ? Number(demoParams.get("demoMaxTokens")) : undefined;
  const demoTemperature = demoParams.has("demoTemp") ? Number(demoParams.get("demoTemp")) : undefined;

  // If no streamed token arrives for this long, something is stuck (a
  // stalled upstream, a dropped connection) — tell the user instead of
  // leaving the streaming cursor spinning forever. This never aborts the
  // request on its own: a real model can legitimately take a while, so
  // only the Stop button cancels it.
  const STALL_TIMEOUT_MS = 20000;

  function clearStallTimer() {
    if (stallTimer) clearTimeout(stallTimer);
    stallTimer = null;
  }

  function armStallTimer() {
    clearStallTimer();
    stallTimer = setTimeout(() => {
      stalled = true;
    }, STALL_TIMEOUT_MS);
  }

  function stopGeneration() {
    abortController?.abort();
  }

  onMount(() => {
    if (initialText || initialAttachments.length > 0) {
      sendMessage(initialText, initialAttachments);
    }
  });

  function stopPlayback() {
    if (currentAudio) {
      currentAudio.pause();
      currentAudio = null;
    }
  }

  async function speak(text: string) {
    stopPlayback();
    try {
      const blob = await api.synthesizeSpeech(text);
      const audio = new Audio(URL.createObjectURL(blob));
      currentAudio = audio;
      audio.play();
    } catch (err) {
      alert(`Speech synthesis: ${err}`);
    }
  }

  function onSend(e: CustomEvent<{ text: string; attachments: { file_name: string; kind: string; text: string }[] }>) {
    sendMessage(e.detail.text, e.detail.attachments);
  }

  // Streams one completion for `history` and appends the result as a new
  // assistant message. Shared by sendMessage (history = everything plus
  // the just-added user turn) and regenerateLastReply (history = up to
  // and including the existing last user turn, no new user message
  // added) -- both just want "stream a reply, append it."
  async function runCompletion(history: { role: string; content: string }[]) {
    streaming = true;
    stalled = false;
    liveReply = "";
    abortController = new AbortController();
    armStallTimer();
    try {
      await api.streamChat(
        model,
        history,
        (delta) => {
          liveReply += delta;
          stalled = false;
          armStallTimer();
        },
        abortController.signal,
        { maxTokens: demoMaxTokens, temperature: demoTemperature }
      );
      await api.addMessage(conversationId, "assistant", liveReply);
    } catch (e) {
      if (abortController?.signal.aborted) {
        // User-initiated Stop: keep whatever was generated so far, no error noise.
        if (liveReply) await api.addMessage(conversationId, "assistant", liveReply);
      } else {
        liveReply += `\n\n[Error: ${e}]`;
        await api.addMessage(conversationId, "assistant", liveReply);
      }
    } finally {
      clearStallTimer();
      streaming = false;
      stalled = false;
      liveReply = "";
      abortController = null;
      onMessagesChanged();
    }
  }

  function toApiHistory(msgs: Message[]): { role: string; content: string }[] {
    return msgs.map((m) => ({ role: m.Role, content: m.Content }));
  }

  // Easter egg 3/3: a hidden phrase, never advertised in the UI. Typed
  // as the entire message (no attachments), it's answered locally --
  // no model call -- with one honest, true line instead of the model's
  // usual reply. Still written through the real addMessage path, so it
  // shows up in history/export like any other message rather than
  // needing separate ephemeral-state plumbing.
  const SECRET_PHRASE = "runyx secret";

  async function sendMessage(text: string, attachments: { file_name: string; kind: string; text: string }[]) {
    if (!text && attachments.length === 0) return;

    if (attachments.length === 0 && text.trim().toLowerCase() === SECRET_PHRASE) {
      stopPlayback();
      await api.addMessage(conversationId, "user", text);
      await api.addMessage(
        conversationId,
        "assistant",
        "Secret found. Runyx runs entirely on your machine -- no cloud, no accounts, no telemetry by default. That's not a clever reply, it's just true."
      );
      onMessagesChanged();
      return;
    }

    let fullContent = text;
    for (const a of attachments) {
      fullContent += `\n\n[Attachment: ${a.file_name}]\n${a.text}`;
    }

    stopPlayback();
    await api.addMessage(conversationId, "user", fullContent);
    onMessagesChanged();
    await runCompletion([...toApiHistory(messages), { role: "user", content: fullContent }]);
  }

  // Re-asks the model for a fresh answer to the last user turn, appended
  // as a new message below the existing reply (the existing reply isn't
  // deleted or hidden -- there's no delete-message endpoint, and
  // overwriting history silently would be a worse surprise than one
  // extra message in the transcript).
  $: lastAssistantIndex = messages.reduce((acc, m, i) => (m.Role === "assistant" ? i : acc), -1);
  $: canRegenerate = !streaming && lastAssistantIndex >= 0;

  async function regenerateLastReply() {
    if (streaming || lastAssistantIndex < 0) return;
    stopPlayback();
    regenerating = true;
    try {
      const upToLastUserTurn = messages.slice(0, lastAssistantIndex);
      await runCompletion(toApiHistory(upToLastUserTurn));
    } finally {
      regenerating = false;
    }
  }
</script>

<div class="chat">
  {#if conversationTitle}
    <div class="chat-header">
      <span class="chat-title">{conversationTitle}</span>
      <button class="export-btn" title="Export as Markdown" aria-label="Export conversation as Markdown" on:click={exportConversation} disabled={exporting}>
        <Download size={14} aria-hidden="true" />
        {exporting ? "Exporting..." : "Export"}
      </button>
    </div>
  {/if}
  <div class="messages" bind:this={messagesEl} on:scroll={onMessagesScroll}>
    {#each messages as m, i (m.ID)}
      <div class="msg {m.Role}" in:fly={{ y: 10, duration: messageFlyDuration, easing: cubicOut }}>
        <div class="bubble">
          {m.Content}
          <div class="msg-actions">
            {#if m.CreatedAt}
              <span class="timestamp" title={formatTimestamp(m.CreatedAt)}>{formatTimestamp(m.CreatedAt)}</span>
            {/if}
            <button class="icon-btn" title="Copy message" aria-label="Copy message" on:click={() => copyMessage(m.ID, m.Content)}>
              {#if copiedId === m.ID}
                <Check size={13} aria-hidden="true" />
              {:else}
                <Copy size={13} aria-hidden="true" />
              {/if}
            </button>
            {#if m.Role === "assistant"}
              <button class="icon-btn" title="Listen to reply" aria-label="Listen to reply" on:click={() => speak(m.Content)}>
                <Volume2 size={13} aria-hidden="true" />
              </button>
            {/if}
          </div>
        </div>
      </div>
      {#if m.Role === "assistant" && i === lastAssistantIndex && canRegenerate}
        <div class="regenerate-row">
          <button class="regenerate-btn" type="button" on:click={regenerateLastReply} disabled={regenerating}>
            <RefreshCw size={13} aria-hidden="true" class={regenerating ? "spin" : ""} />
            {regenerating ? "Regenerating..." : "Regenerate"}
          </button>
        </div>
      {/if}
    {/each}
    {#if streaming}
      <div class="msg assistant" in:fly={{ y: 10, duration: messageFlyDuration, easing: cubicOut }}>
        <div class="bubble">
          {liveReply}<span class="cursor">|</span>
          {#if stalled}
            <div class="stall-notice">
              <span>Model is slow or stalled</span>
              <button class="stop-btn" type="button" on:click={stopGeneration}>Stop</button>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>

  {#if showScrollToBottom}
    <button
      class="scroll-to-bottom"
      type="button"
      title="Scroll to latest"
      aria-label="Scroll to latest message"
      on:click={onScrollToBottomClick}
      transition:fly={{ y: 10, duration: reduceMotion ? 0 : 160 }}
    >
      <ArrowDown size={16} aria-hidden="true" />
    </button>
  {/if}

  <div class="composer-dock">
    <Composer {installedModels} bind:model {onBrowseMore} disabled={streaming} on:send={onSend} {reduceMotion} />
  </div>
</div>

<style>
  .chat {
    position: relative;
    flex: 1;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
  }
  .chat-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.75rem 1.5rem;
    border-bottom: 1px solid var(--border);
    flex: 0 0 auto;
  }
  .chat-title {
    font-weight: 600;
    font-size: 0.92rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .export-btn {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-muted);
    border-radius: var(--radius-md);
    padding: 0.35rem 0.75rem;
    font-size: 0.8rem;
    flex: 0 0 auto;
  }
  .export-btn:hover:not(:disabled) {
    background: var(--bg-sunken);
    color: var(--text);
  }
  .export-btn:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 2rem 1.5rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    max-width: 720px;
    width: 100%;
    margin: 0 auto;
  }
  .msg {
    display: flex;
  }
  .msg.user {
    justify-content: flex-end;
  }
  .bubble {
    max-width: 85%;
    padding: 0.7rem 1rem;
    border-radius: var(--radius-lg);
    white-space: pre-wrap;
    line-height: 1.5;
  }
  .msg.user .bubble {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .msg.assistant .bubble {
    max-width: 100%;
    padding: 0;
    background: none;
    border: none;
  }
  .cursor {
    animation: runyx-pulse 1s ease-in-out infinite;
  }
  .stall-notice {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.5rem;
    font-size: 0.85rem;
    opacity: 0.8;
  }
  .stop-btn {
    border: 1px solid var(--border);
    background: transparent;
    color: inherit;
    border-radius: var(--radius-sm, 4px);
    padding: 0.15rem 0.6rem;
    cursor: pointer;
    font-size: 0.8rem;
  }
  .stop-btn:hover {
    background: var(--bg-elevated);
  }
  .msg-actions {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    margin-top: 0.3rem;
    opacity: 0.55;
    transition: opacity var(--transition-fast);
  }
  .msg:hover .msg-actions,
  .msg:focus-within .msg-actions {
    opacity: 1;
  }
  .msg.user .msg-actions {
    justify-content: flex-end;
  }
  .timestamp {
    font-size: 0.72rem;
    color: var(--text-muted);
    margin-right: 0.2rem;
  }
  .msg.user .timestamp {
    color: color-mix(in srgb, var(--accent-contrast) 70%, transparent);
  }
  .icon-btn {
    border: none;
    background: transparent;
    color: inherit;
    opacity: 0.6;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    padding: 0.1rem;
  }
  .icon-btn:hover {
    opacity: 1;
  }
  .regenerate-row {
    display: flex;
    justify-content: flex-start;
    margin-top: -0.5rem;
  }
  .regenerate-btn {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-muted);
    border-radius: 999px;
    padding: 0.3rem 0.75rem;
    font-size: 0.78rem;
  }
  .regenerate-btn:hover:not(:disabled) {
    background: var(--bg-sunken);
    color: var(--text);
  }
  .regenerate-btn:disabled {
    opacity: 0.6;
    cursor: default;
  }
  :global(.regenerate-btn .spin) {
    animation: runyx-spin 0.9s linear infinite;
  }
  @keyframes runyx-spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }
  .scroll-to-bottom {
    position: absolute;
    left: 50%;
    bottom: 6.5rem;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.2rem;
    height: 2.2rem;
    border-radius: 999px;
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    color: var(--text);
    box-shadow: var(--shadow);
  }
  .scroll-to-bottom:hover {
    background: var(--bg-sunken);
  }
  .composer-dock {
    padding: 0 1.5rem 1.5rem;
  }
</style>
