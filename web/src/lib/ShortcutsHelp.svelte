<script lang="ts">
  import { fade, fly } from "svelte/transition";
  import { X, Keyboard } from "./icons";

  export let open = false;
  export let reduceMotion = false;

  $: fadeDuration = reduceMotion ? 0 : 140;
  $: flyDuration = reduceMotion ? 0 : 200;

  const isMac = typeof navigator !== "undefined" && /Mac/.test(navigator.platform);
  const mod = isMac ? "Cmd" : "Ctrl";

  const shortcuts: { keys: string; desc: string }[] = [
    { keys: `${mod} K`, desc: "Open the command palette" },
    { keys: `${mod} Shift F`, desc: "Search every message" },
    { keys: `${mod} Shift O`, desc: "Start a new chat" },
    { keys: `${mod} /`, desc: "Show this shortcuts list" },
    { keys: "Enter", desc: "Send the message" },
    { keys: "Shift Enter", desc: "New line without sending" },
    { keys: "Esc", desc: "Close a palette, dialog or menu" },
  ];

  function close() {
    open = false;
  }

  // A window-level listener, not one on the panel itself: nothing
  // inside this dialog is focused (no input to autofocus), so a
  // listener scoped to the panel's own subtree would never see the
  // Escape keydown -- it goes to whatever already had focus, usually
  // outside the panel entirely.
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
      aria-label="Keyboard shortcuts"
      on:click|stopPropagation
      on:keydown={onKeydown}
      transition:fly={{ y: -10, duration: flyDuration }}
    >
      <div class="head">
        <Keyboard size={16} aria-hidden="true" />
        <span>Keyboard shortcuts</span>
        <button class="close-btn" aria-label="Close" on:click={close}>
          <X size={15} aria-hidden="true" />
        </button>
      </div>
      <ul>
        {#each shortcuts as s}
          <li>
            <span class="desc">{s.desc}</span>
            <span class="keys">
              {#each s.keys.split(" ") as k}<kbd>{k}</kbd>{/each}
            </span>
          </li>
        {/each}
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
    align-items: center;
    z-index: 100;
  }
  .panel {
    width: min(420px, 90vw);
    background: var(--bg-elevated);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow);
    border: 1px solid var(--border);
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.9rem 1.1rem;
    border-bottom: 1px solid var(--border);
    font-weight: 600;
    font-size: 0.92rem;
  }
  .close-btn {
    margin-left: auto;
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
  ul {
    list-style: none;
    margin: 0;
    padding: 0.5rem 0;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.5rem 1.1rem;
    font-size: 0.86rem;
  }
  .desc {
    color: var(--text);
  }
  .keys {
    display: flex;
    gap: 0.25rem;
    flex: 0 0 auto;
  }
  kbd {
    font-family: var(--font-mono);
    font-size: 0.74rem;
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.1rem 0.4rem;
    color: var(--text-muted);
  }
</style>
