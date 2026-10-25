<script lang="ts">
  import { fade, fly } from "svelte/transition";
  import { api, type Conversation, type Message } from "./api";
  import { Search, X } from "./icons";

  export let open = false;
  export let conversations: Conversation[] = [];
  export let onJump: (conversationId: string) => void;
  export let reduceMotion = false;

  $: fadeDuration = reduceMotion ? 0 : 140;
  $: flyDuration = reduceMotion ? 0 : 200;

  let query = "";
  let results: Message[] = [];
  let searching = false;
  let searchedOnce = false;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  function titleFor(conversationId: string): string {
    const c = conversations.find((c) => c.ID === conversationId);
    return c?.Title || c?.Model || "Untitled conversation";
  }

  function snippet(content: string, q: string): string {
    const lower = content.toLowerCase();
    const idx = lower.indexOf(q.toLowerCase());
    if (idx < 0) return content.slice(0, 140);
    const start = Math.max(0, idx - 40);
    const end = Math.min(content.length, idx + q.length + 80);
    return (start > 0 ? "..." : "") + content.slice(start, end) + (end < content.length ? "..." : "");
  }

  async function runSearch() {
    const q = query.trim();
    if (!q) {
      results = [];
      searchedOnce = false;
      return;
    }
    searching = true;
    try {
      results = (await api.search(q).catch(() => [])) ?? [];
    } finally {
      searching = false;
      searchedOnce = true;
    }
  }

  function onInput() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(runSearch, 250);
  }

  function jump(m: Message) {
    onJump(m.ConversationID);
    close();
  }

  function close() {
    open = false;
    query = "";
    results = [];
    searchedOnce = false;
  }

  // Window-level, not just on the input: a click on a result or
  // anywhere else in the panel moves focus off the input, and Escape
  // should still close the panel from there.
  function onKeydown(e: KeyboardEvent) {
    if (open && e.key === "Escape") close();
  }
</script>

<svelte:window on:keydown={onKeydown} />

{#if open}
  <div class="overlay" role="presentation" on:click={close} transition:fade={{ duration: fadeDuration }}>
    <div
      class="panel"
      role="dialog"
      aria-label="Search messages"
      on:click|stopPropagation
      transition:fly={{ y: -10, duration: flyDuration }}
    >
      <div class="search-row">
        <Search size={15} aria-hidden="true" />
        <input
          autofocus
          placeholder="Search every message, across every conversation..."
          bind:value={query}
          on:input={onInput}
          on:keydown={onKeydown}
        />
        <button class="close-btn" aria-label="Close search" on:click={close}>
          <X size={15} aria-hidden="true" />
        </button>
      </div>

      <div class="results">
        {#if searching}
          <p class="status">Searching...</p>
        {:else if searchedOnce && results.length === 0}
          <p class="status">No messages match "{query}".</p>
        {/if}
        {#each results as m (m.ID)}
          <button class="result" on:click={() => jump(m)}>
            <div class="result-head">
              <span class="result-title">{titleFor(m.ConversationID)}</span>
              <span class="result-role">{m.Role}</span>
            </div>
            <p class="result-snippet">{snippet(m.Content, query)}</p>
          </button>
        {/each}
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(20, 18, 15, 0.35);
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 10vh;
    z-index: 100;
  }
  .panel {
    width: min(640px, 92vw);
    max-height: 70vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-elevated);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow);
    border: 1px solid var(--border);
    overflow: hidden;
  }
  .search-row {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.9rem 1.1rem;
    border-bottom: 1px solid var(--border);
    color: var(--text-muted);
    flex: 0 0 auto;
  }
  .search-row input {
    flex: 1;
    border: none;
    background: transparent;
    outline: none;
    color: var(--text);
    font-size: 0.95rem;
  }
  .close-btn {
    border: none;
    background: transparent;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    padding: 0.2rem;
  }
  .close-btn:hover {
    color: var(--text);
  }
  .results {
    overflow-y: auto;
    padding: 0.5rem;
  }
  .status {
    padding: 1rem;
    color: var(--text-muted);
    font-size: 0.88rem;
    margin: 0;
  }
  .result {
    display: block;
    width: 100%;
    text-align: left;
    border: none;
    background: transparent;
    border-radius: var(--radius-sm);
    padding: 0.6rem 0.75rem;
    margin-bottom: 0.15rem;
  }
  .result:hover {
    background: var(--bg-sunken);
  }
  .result-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }
  .result-title {
    font-weight: 600;
    font-size: 0.86rem;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .result-role {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-muted);
    flex: 0 0 auto;
  }
  .result-snippet {
    margin: 0.2rem 0 0;
    font-size: 0.82rem;
    color: var(--text-muted);
    line-height: 1.4;
  }
</style>
