<script lang="ts">
  import { onMount } from "svelte";
  import { api, type PermissionsSnapshot } from "./api";
  import { ShieldCheck, Smartphone } from "./icons";

  export let onPairDevice: () => void;

  let perms: PermissionsSnapshot | null = null;

  async function refresh() {
    perms = await api.permissions().catch(() => null);
  }

  async function revoke(id: string) {
    await api.revokeDevice(id);
    await refresh();
  }

  onMount(refresh);
</script>

<div class="page">
  <h1>Agent &amp; permissions</h1>

  {#if perms}
    <section>
      <h2>Shell agent</h2>
      <p class="status">
        <ShieldCheck size={14} aria-hidden="true" />
        {perms.shell_agent.enabled ? "Enabled" : "Disabled"}
      </p>
      {#if perms.shell_agent.enabled}
        <p class="muted">Working directory: {perms.shell_agent.work_dir}</p>
      {/if}
    </section>

    <section>
      <h2>Capabilities</h2>
      <ul class="flags">
        <li>Vision: {perms.vision_enabled ? "On" : "Off"}</li>
        <li>Voice: {perms.voice_enabled ? "On" : "Off"}</li>
        <li>Web search: {perms.web_search_on ? "On" : "Off"}</li>
      </ul>
    </section>

    <section>
      <div class="section-header">
        <h2>Paired devices</h2>
        <button class="pair-btn" on:click={onPairDevice}>
          <Smartphone size={14} aria-hidden="true" />
          Pair a device
        </button>
      </div>
      {#if (perms.devices ?? []).length === 0}
        <p class="muted">No paired devices.</p>
      {:else}
        <ul class="devices">
          {#each perms.devices as d}
            <li>
              <span>{d.name}</span>
              <button on:click={() => revoke(d.id)}>Revoke</button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/if}
</div>

<style>
  .page {
    flex: 1;
    overflow-y: auto;
    padding: 2rem 2.5rem;
    max-width: 640px;
  }
  h1 {
    margin: 0 0 1.5rem;
  }
  section {
    margin-bottom: 2rem;
  }
  h2 {
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    font-weight: 500;
    margin: 0 0 0.75rem;
  }
  .status {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--text-muted);
  }
  .muted {
    color: var(--text-muted);
  }
  .flags {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    gap: 1.5rem;
    font-size: 0.9rem;
  }
  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .pair-btn {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    border-radius: var(--radius-md);
    padding: 0.35rem 0.7rem;
    font-size: 0.8rem;
    color: var(--text);
  }
  .devices {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .devices li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--border);
  }
  .devices button {
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-muted);
    border-radius: var(--radius-sm);
    padding: 0.3rem 0.6rem;
    font-size: 0.8rem;
  }
</style>
