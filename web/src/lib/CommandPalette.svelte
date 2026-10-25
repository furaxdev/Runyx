<script lang="ts">
  import { fade, fly } from "svelte/transition";
  import { flip } from "svelte/animate";
  import { t, type Lang } from "./i18n";

  export let lang: Lang;
  export let open = false;
  export let commands: { label: string; run: () => void }[] = [];
  export let reduceMotion = false;

  $: fadeDuration = reduceMotion ? 0 : 140;
  $: flyDuration = reduceMotion ? 0 : 200;

  let query = "";
  let filtered: typeof commands = [];
  $: filtered = commands.filter((c) => c.label.toLowerCase().includes(query.toLowerCase()));

  function runAndClose(fn: () => void) {
    fn();
    open = false;
    query = "";
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      open = false;
    }
  }
</script>

{#if open}
  <div class="overlay" on:click={() => (open = false)} role="presentation" transition:fade={{ duration: fadeDuration }}>
    <div
      class="palette"
      on:click|stopPropagation
      role="dialog"
      aria-label={t(lang, "commandPalette")}
      transition:fly={{ y: -10, duration: flyDuration, easing: (t) => 1 - Math.pow(1 - t, 3) }}
    >
      <input
        autofocus
        placeholder={t(lang, "search")}
        bind:value={query}
        on:keydown={onKeydown}
      />
      <ul>
        {#each filtered as cmd (cmd.label)}
          <li animate:flip={{ duration: reduceMotion ? 0 : 180 }}>
            <button on:click={() => runAndClose(cmd.run)}>{cmd.label}</button>
          </li>
        {/each}
        {#if filtered.length === 0}
          <li class="empty">—</li>
        {/if}
      </ul>
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
    padding-top: 12vh;
    z-index: 100;
  }
  .palette {
    width: min(520px, 90vw);
    background: var(--bg-elevated);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow);
    border: 1px solid var(--border);
    overflow: hidden;
  }
  input {
    width: 100%;
    border: none;
    background: transparent;
    padding: 1rem 1.25rem;
    font-size: 1rem;
    color: var(--text);
    border-bottom: 1px solid var(--border);
    outline: none;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0.5rem;
    max-height: 320px;
    overflow-y: auto;
  }
  li.empty {
    padding: 0.75rem 1rem;
    color: var(--text-muted);
  }
  button {
    display: block;
    width: 100%;
    text-align: left;
    padding: 0.6rem 0.9rem;
    border: none;
    background: transparent;
    color: var(--text);
    border-radius: var(--radius-sm);
    font-size: 0.95rem;
  }
  button:hover {
    background: var(--bg-sunken);
  }
</style>
