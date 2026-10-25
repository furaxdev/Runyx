<script lang="ts">
  import { onMount } from "svelte";
  import { api, type RegistryModel } from "./api";
  import { activeDownloads } from "./downloads";
  import ModelCard from "./ModelCard.svelte";
  import { Search } from "./icons";

  export let onUse: (pullSpec: string) => void;

  let all: RegistryModel[] = [];
  let query = "";
  let capFilter = "";
  let sort: "recommended" | "size" | "recent" = "recommended";
  let errorsByID: Record<string, string> = {};
  let abortControllers: Record<string, AbortController> = {};

  async function refresh() {
    all = await api.registry().catch(() => []);
  }
  onMount(refresh);

  async function download(m: RegistryModel) {
    const { [m.id]: _, ...rest } = errorsByID;
    errorsByID = rest;
    const controller = new AbortController();
    abortControllers[m.id] = controller;
    activeDownloads.update((d) => ({ ...d, [m.id]: { id: m.id, name: m.name, downloaded: 0, total: m.size_bytes } }));
    try {
      await api.pull(
        { id: m.id },
        (p) => {
          if (p.error) {
            errorsByID = { ...errorsByID, [m.id]: p.error };
            return;
          }
          activeDownloads.update((d) => ({
            ...d,
            [m.id]: { id: m.id, name: m.name, downloaded: p.downloaded ?? 0, total: p.total ?? m.size_bytes },
          }));
          if (p.done) {
            activeDownloads.update((d) => {
              const { [m.id]: _unused, ...rest } = d;
              return rest;
            });
            refresh();
          }
        },
        controller.signal
      );
    } catch (err: any) {
      if (err?.name !== "AbortError") {
        errorsByID = { ...errorsByID, [m.id]: String(err) };
      }
      activeDownloads.update((d) => {
        const { [m.id]: _unused, ...rest } = d;
        return rest;
      });
    }
  }

  function pauseDownload(id: string) {
    abortControllers[id]?.abort();
  }

  async function remove(m: RegistryModel) {
    if (!confirm(`Remove ${m.name}?`)) return;
    await api.deleteModel(m.pull_spec).catch(() => {});
    await refresh();
  }

  $: filtered = all.filter((m) => {
    if (
      query &&
      !m.name.toLowerCase().includes(query.toLowerCase()) &&
      !m.description.toLowerCase().includes(query.toLowerCase())
    )
      return false;
    if (capFilter && !m.capabilities.includes(capFilter)) return false;
    return true;
  });

  $: installed = filtered.filter((m) => m.installed);
  $: recommended = filtered
    .filter((m) => !m.installed && (m.fit === "comfortable" || m.fit === "ok"))
    .sort((a, b) => b.params_b - a.params_b)
    .slice(0, 6);
  $: browseAll = (() => {
    const list = [...filtered];
    if (sort === "size") list.sort((a, b) => a.size_bytes - b.size_bytes);
    else if (sort === "recent") list.reverse();
    else list.sort((a, b) => (a.fit === b.fit ? b.params_b - a.params_b : a.fit === "comfortable" ? -1 : 1));
    return list;
  })();

  const capabilityOptions: [string, string][] = [
    ["", "All capabilities"],
    ["vision", "Vision"],
    ["tools", "Tools"],
    ["reasoning", "Reasoning"],
    ["code", "Code"],
    ["multilingual", "Multilingual"],
    ["long_context", "Long context"],
  ];

  let advancedRepo = "";
  let advancedFile = "";
  let advancedBusy = false;
  let advancedMessage = "";

  async function advancedPull() {
    if (!advancedRepo || !advancedFile) return;
    advancedBusy = true;
    advancedMessage = "";
    try {
      await api.pull({ repo: advancedRepo, file: advancedFile }, (p) => {
        if (p.error) advancedMessage = p.error;
        if (p.done && !p.error) advancedMessage = "Done.";
      });
      await refresh();
    } catch (err) {
      advancedMessage = String(err);
    } finally {
      advancedBusy = false;
    }
  }
</script>

<div class="page">
  <h1>Models</h1>

  <div class="toolbar">
    <div class="search-box">
      <Search size={14} aria-hidden="true" />
      <input placeholder="Search models" bind:value={query} />
    </div>
    <select bind:value={capFilter}>
      {#each capabilityOptions as [v, l]}
        <option value={v}>{l}</option>
      {/each}
    </select>
    <select bind:value={sort}>
      <option value="recommended">Sort: recommended</option>
      <option value="size">Sort: size</option>
      <option value="recent">Sort: recently added</option>
    </select>
  </div>

  {#if installed.length > 0}
    <section>
      <h2>Installed</h2>
      <div class="cards">
        {#each installed as m (m.id)}
          <ModelCard
            {m}
            {onUse}
            onDownload={download}
            onPause={pauseDownload}
            onDelete={remove}
            error={errorsByID[m.id] ?? ""}
            showUse
          />
        {/each}
      </div>
    </section>
  {/if}

  {#if recommended.length > 0}
    <section>
      <h2>Recommended for your machine</h2>
      <div class="cards">
        {#each recommended as m (m.id)}
          <ModelCard {m} {onUse} onDownload={download} onPause={pauseDownload} onDelete={remove} error={errorsByID[m.id] ?? ""} />
        {/each}
      </div>
    </section>
  {/if}

  <section>
    <h2>Browse all</h2>
    <div class="cards">
      {#each browseAll as m (m.id)}
        <ModelCard {m} {onUse} onDownload={download} onPause={pauseDownload} onDelete={remove} error={errorsByID[m.id] ?? ""} showUse />
      {/each}
    </div>
  </section>

  <details class="advanced">
    <summary>Advanced: pull by repo</summary>
    <div class="advanced-form">
      <input placeholder="owner/repo" bind:value={advancedRepo} />
      <input placeholder="file.gguf" bind:value={advancedFile} />
      <button class="btn primary" disabled={advancedBusy} on:click={advancedPull}>Pull</button>
    </div>
    {#if advancedMessage}<p class="msg">{advancedMessage}</p>{/if}
  </details>
</div>

<style>
  .page {
    flex: 1;
    overflow-y: auto;
    padding: 2rem 2.5rem;
    max-width: 920px;
  }
  h1 {
    margin: 0 0 1.25rem;
  }
  .toolbar {
    display: flex;
    gap: 0.6rem;
    margin-bottom: 1.5rem;
    flex-wrap: wrap;
  }
  .search-box {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    border-radius: var(--radius-md);
    padding: 0.45rem 0.7rem;
    flex: 1;
    min-width: 180px;
    color: var(--text-muted);
  }
  .search-box input {
    border: none;
    background: transparent;
    outline: none;
    color: var(--text);
    width: 100%;
  }
  select {
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    color: var(--text);
    border-radius: var(--radius-md);
    padding: 0.45rem 0.6rem;
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
  .cards {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .advanced {
    border-top: 1px solid var(--border);
    padding-top: 1rem;
    color: var(--text-muted);
    font-size: 0.85rem;
  }
  .advanced summary {
    cursor: pointer;
  }
  .advanced-form {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }
  .advanced-form input {
    flex: 1;
    padding: 0.5rem 0.7rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--bg-elevated);
    color: var(--text);
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
  .msg {
    margin-top: 0.5rem;
  }
</style>
