<script lang="ts">
  import type { RegistryModel } from "./api";
  import { activeDownloads } from "./downloads";
  import Badges from "./Badges.svelte";
  import { Download, Pause, Trash2, Check, modelFamilyIcon } from "./icons";

  export let m: RegistryModel;
  export let onUse: (pullSpec: string) => void;
  export let onDownload: (m: RegistryModel) => void;
  export let onPause: (id: string) => void;
  export let onDelete: (m: RegistryModel) => void;
  export let error = "";
  export let showUse = false;

  function fitLabel(fit: string): string {
    switch (fit) {
      case "comfortable":
        return "Runs well on your machine";
      case "ok":
        return "Runs on your machine";
      case "tight":
        return "Tight fit";
      default:
        return "Too large";
    }
  }

  function fmtSize(bytes: number): string {
    return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
  }
</script>

<div class="card">
  <div class="card-main">
    <div class="card-title-row">
      <span class="family-icon" title={m.family || m.publisher}>
        <svelte:component this={modelFamilyIcon(m.family || m.name)} size={16} aria-hidden="true" />
      </span>
      <span class="name">{m.name}</span>
      <span class="publisher">{m.publisher}</span>
    </div>
    <p class="desc">{m.description}</p>
    <Badges capabilities={m.capabilities} />
    <a class="hf-link" href={m.hf_url} target="_blank" rel="noreferrer">Model card &amp; license ({m.license})</a>
  </div>
  <div class="card-side">
    <span class="size">{fmtSize(m.size_bytes)}</span>
    {#if !m.installed}
      <span class="fit fit-{m.fit}">{fitLabel(m.fit)}</span>
    {/if}
    {#if m.installed}
      <span class="installed-tag"><Check size={13} aria-hidden="true" /> Installed</span>
      <div class="actions">
        {#if showUse}
          <button class="btn primary" on:click={() => onUse(m.pull_spec)}>Use</button>
        {/if}
        <button class="btn danger" aria-label="Delete" on:click={() => onDelete(m)}>
          <Trash2 size={14} aria-hidden="true" />
        </button>
      </div>
    {:else if $activeDownloads[m.id]}
      {@const d = $activeDownloads[m.id]}
      <div class="progress">
        <div class="bar" style="width: {d.total ? Math.min(100, (d.downloaded / d.total) * 100) : 0}%" />
      </div>
      <button class="btn" on:click={() => onPause(m.id)}>
        <Pause size={13} aria-hidden="true" /> Pause
      </button>
    {:else}
      <button class="btn primary" on:click={() => onDownload(m)}>
        <Download size={13} aria-hidden="true" /> Download
      </button>
    {/if}
    {#if error}
      <p class="error">{error}</p>
    {/if}
  </div>
</div>

<style>
  .card {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    border-radius: var(--radius-lg);
    padding: 1rem 1.2rem;
  }
  .card-main {
    flex: 1;
    min-width: 0;
  }
  .card-title-row {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
    margin-bottom: 0.3rem;
  }
  .family-icon {
    display: flex;
    align-items: center;
    color: var(--text-muted);
    flex: 0 0 auto;
  }
  .name {
    font-weight: 600;
    color: var(--text);
  }
  .publisher {
    font-size: 0.78rem;
    color: var(--text-muted);
  }
  .desc {
    font-size: 0.85rem;
    color: var(--text-muted);
    margin: 0 0 0.5rem;
  }
  .hf-link {
    display: inline-block;
    margin-top: 0.5rem;
    font-size: 0.78rem;
    color: var(--accent);
  }
  .card-side {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 0.4rem;
    min-width: 140px;
  }
  .size {
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .fit {
    font-size: 0.72rem;
    border-radius: 999px;
    padding: 0.15rem 0.55rem;
    background: var(--bg-sunken);
    color: var(--text-muted);
  }
  .fit-comfortable {
    background: color-mix(in srgb, var(--accent) 18%, transparent);
    color: var(--accent);
  }
  .installed-tag {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.78rem;
    color: var(--text-muted);
  }
  .btn {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    border: 1px solid var(--border);
    background: transparent;
    border-radius: var(--radius-md);
    padding: 0.4rem 0.7rem;
    font-size: 0.82rem;
    color: var(--text);
  }
  .btn.primary {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: transparent;
  }
  .btn.danger {
    color: var(--text-muted);
  }
  .actions {
    display: flex;
    gap: 0.4rem;
  }
  .progress {
    width: 120px;
    height: 6px;
    background: var(--bg-sunken);
    border-radius: 999px;
    overflow: hidden;
  }
  .bar {
    height: 100%;
    background: var(--accent);
  }
  .error {
    font-size: 0.75rem;
    color: var(--danger, #c0392b);
    max-width: 160px;
    text-align: right;
  }
</style>
