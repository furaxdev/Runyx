<script lang="ts">
  import { onMount } from "svelte";
  import { api, type UpdateCheckResult, type GPUSaverStatus } from "./api";

  export let theme: "system" | "light" | "dark";
  export let reduceMotion: boolean;
  export let onSetTheme: (v: "system" | "light" | "dark") => void;
  export let onToggleReduceMotion: (v: boolean) => void;

  function onThemeChange(e: Event) {
    const v = (e.target as HTMLSelectElement).value;
    onSetTheme(v === "light" || v === "dark" ? v : "system");
  }

  function onReduceMotionChange(e: Event) {
    onToggleReduceMotion((e.target as HTMLInputElement).checked);
  }

  // The daemon checks for a newer release itself, in the background,
  // every few hours (see internal/update.Checker) -- this just reads
  // its cached result, never triggering a live GitHub call from the
  // browser. It only ever reports; it never downloads or installs
  // anything on its own (see docs/DECISIONS.md on why automatic apply
  // stays refused until a real release signing key exists).
  let updateInfo: UpdateCheckResult | null = null;
  let gpuSaver: GPUSaverStatus | null = null;
  onMount(async () => {
    updateInfo = await api.checkUpdate().catch(() => null);
    gpuSaver = await api.gpuSaverStatus().catch(() => null);
  });
</script>

<div class="page">
  <h1>Settings</h1>

  <section>
    <h2>Appearance</h2>
    <div class="row">
      <label for="theme-select">Theme</label>
      <select id="theme-select" value={theme} on:change={onThemeChange}>
        <option value="system">System</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
    </div>
    <div class="row">
      <label>
        <input type="checkbox" checked={reduceMotion} on:change={onReduceMotionChange} />
        Reduce motion
      </label>
    </div>
  </section>

  <section>
    <h2>GPU Saver</h2>
    {#if gpuSaver}
      {#if gpuSaver.enabled}
        <p class="version-row">
          Enabled. colibri is {gpuSaver.available ? "fetched and ready" : "not fetched yet"}.
        </p>
        {#if !gpuSaver.available}
          <p class="version-row muted">
            Run <code>runyx gpu-saver import &lt;model&gt;</code> once for a model you want served this way.
          </p>
        {/if}
      {:else}
        <p class="version-row muted">
          Off. For very large MoE models that don't fit in RAM/VRAM, GPU Saver streams unused experts from disk
          instead (via <a href="https://github.com/JustVugg/colibri" target="_blank" rel="noreferrer">colibri</a>).
          Turn it on with <code>gpu_saver_enabled = true</code> in config.toml, then
          <code>runyx gpu-saver import &lt;model&gt;</code> for a recommended model.
        </p>
      {/if}
    {/if}
  </section>

  <section>
    <h2>About</h2>
    {#if updateInfo}
      <p class="version-row">Current version: {updateInfo.current_version}</p>
      {#if updateInfo.update_available}
        <div class="update-banner">
          <span>Runyx {updateInfo.latest_version} is available.</span>
          {#if updateInfo.download_url}
            <a href={updateInfo.download_url} target="_blank" rel="noreferrer">Download</a>
          {/if}
        </div>
      {:else if !updateInfo.error}
        <p class="version-row muted">You're on the latest version.</p>
      {/if}
    {/if}
  </section>
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
  .row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }
  select {
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--bg-elevated);
    color: var(--text);
  }
  .version-row {
    margin: 0 0 0.5rem;
    font-size: 0.85rem;
  }
  .version-row.muted {
    color: var(--text-muted);
  }
  .version-row code {
    font-family: var(--font-mono);
    background: var(--bg-sunken);
    border-radius: 4px;
    padding: 0.1rem 0.3rem;
  }
  .version-row a {
    color: var(--accent);
  }
  .update-banner {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem 0.9rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--bg-sunken);
    font-size: 0.85rem;
  }
  .update-banner a {
    color: var(--accent);
    font-weight: 600;
    white-space: nowrap;
  }
</style>
