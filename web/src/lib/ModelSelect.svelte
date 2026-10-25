<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import type { RegistryModel } from "./api";
  import { ChevronDown, Check, modelFamilyIcon } from "./icons";
  import Badges from "./Badges.svelte";

  export let installed: RegistryModel[] = [];
  export let selected = "";
  export let onBrowseMore: () => void;

  const dispatch = createEventDispatcher<{ select: string }>();
  let open = false;

  function pick(id: string) {
    selected = id;
    dispatch("select", id);
    open = false;
  }

  $: current = installed.find((m) => m.pull_spec === selected || m.id === selected);

  function toggle() {
    open = !open;
  }

  function onWindowClick(e: MouseEvent) {
    const el = e.target as HTMLElement;
    if (!el.closest(".model-select")) open = false;
  }
</script>

<svelte:window on:click={onWindowClick} />

<div class="model-select">
  <button type="button" class="trigger" on:click|stopPropagation={toggle}>
    {#if current}
      <svelte:component this={modelFamilyIcon(current.family || current.name)} size={14} aria-hidden="true" />
    {/if}
    <span class="label">{current?.name ?? "Select a model"}</span>
    <ChevronDown size={14} aria-hidden="true" />
  </button>

  {#if open}
    <div class="menu" role="listbox">
      {#if installed.length === 0}
        <p class="empty">No models installed yet.</p>
      {:else}
        {#each installed as m}
          <button type="button" class="item" role="option" aria-selected={m.pull_spec === selected} on:click={() => pick(m.pull_spec)}>
            <div class="item-main">
              <span class="item-name">
                <svelte:component this={modelFamilyIcon(m.family || m.name)} size={14} aria-hidden="true" />
                {m.name}
              </span>
              {#if m.pull_spec === selected}
                <Check size={14} aria-hidden="true" />
              {/if}
            </div>
            <p class="item-desc">{m.description}</p>
            <Badges capabilities={m.capabilities} compact />
          </button>
        {/each}
      {/if}
      <button type="button" class="browse-more" on:click={onBrowseMore}>Browse more models</button>
    </div>
  {/if}
</div>

<style>
  .model-select {
    position: relative;
  }
  .trigger {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    border: none;
    background: var(--bg-sunken);
    border-radius: 999px;
    padding: 0.38rem 0.7rem;
    color: var(--text);
    font-size: 0.8rem;
    max-width: 220px;
    transition: background var(--transition-fast);
  }
  .trigger:hover {
    background: color-mix(in srgb, var(--bg-sunken) 70%, var(--text) 8%);
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .menu {
    position: absolute;
    bottom: calc(100% + 0.4rem);
    left: 0;
    width: 320px;
    max-height: 360px;
    overflow-y: auto;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow);
    padding: 0.4rem;
    z-index: 50;
  }
  .empty {
    padding: 0.75rem;
    color: var(--text-muted);
    font-size: 0.85rem;
  }
  .item {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.6rem;
    color: var(--text);
  }
  .item:hover {
    background: var(--bg-sunken);
  }
  .item-main {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.88rem;
    font-weight: 500;
  }
  .item-name {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    min-width: 0;
  }
  .item-name :global(svg) {
    flex: 0 0 auto;
    color: var(--text-muted);
  }
  .item-desc {
    margin: 0.2rem 0 0.35rem;
    font-size: 0.78rem;
    color: var(--text-muted);
  }
  .browse-more {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    border-top: 1px solid var(--border);
    margin-top: 0.3rem;
    padding: 0.6rem;
    color: var(--accent);
    font-size: 0.82rem;
  }

  @media (max-width: 640px) {
    .trigger {
      min-height: 44px;
      max-width: 140px;
    }
  }
</style>
