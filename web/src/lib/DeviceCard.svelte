<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { api, type DashboardStats } from "./api";
  import { activeDownloads } from "./downloads";
  import { Cpu, Download } from "./icons";

  export let collapsed = false;
  export let onOpenDownloads: () => void;

  let stats: DashboardStats | null = null;
  let timer: ReturnType<typeof setInterval>;

  async function refresh() {
    stats = await api.dashboard().catch(() => null);
  }

  onMount(() => {
    refresh();
    timer = setInterval(refresh, 5000);
  });
  onDestroy(() => clearInterval(timer));

  $: downloadCount = Object.keys($activeDownloads).length;
  $: loadedModel = stats?.loaded_models?.[0]?.name ?? "—";
</script>

<div class="device-card" class:collapsed>
  {#if !collapsed}
    <div class="row">
      <Cpu size={15} aria-hidden="true" />
      <span class="label">{stats?.backend ?? "—"}</span>
    </div>
    <div class="row muted">
      <span>RAM {stats ? (stats.total_ram_mb / 1024).toFixed(1) : "—"} GB</span>
      {#if stats && stats.total_vram_mb > 0}
        <span>· VRAM {(stats.total_vram_mb / 1024).toFixed(1)} GB</span>
      {/if}
    </div>
    <div class="row muted">
      <span class="truncate">Loaded: {loadedModel}</span>
    </div>
  {/if}
  {#if downloadCount > 0}
    <button class="downloads-btn" on:click={onOpenDownloads} title="Downloads in progress">
      <Download size={14} aria-hidden="true" />
      {#if !collapsed}<span>{downloadCount} downloading</span>{/if}
    </button>
  {/if}
</div>

<style>
  .device-card {
    border-top: 1px solid var(--border);
    padding: 0.75rem 0.9rem;
    font-size: 0.78rem;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .device-card.collapsed {
    align-items: center;
    padding: 0.75rem 0.4rem;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--text);
  }
  .row.muted {
    color: var(--text-muted);
    gap: 0.5rem;
  }
  .truncate {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .downloads-btn {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    border: 1px solid var(--border);
    background: var(--bg-sunken);
    border-radius: var(--radius-sm);
    padding: 0.35rem 0.5rem;
    color: var(--text);
    margin-top: 0.2rem;
  }
</style>
