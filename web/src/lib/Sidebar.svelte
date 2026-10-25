<script lang="ts">
  import { onMount } from "svelte";
  import { fade } from "svelte/transition";
  import type { Conversation, Project } from "./api";
  import { api } from "./api";
  import Logo from "./Logo.svelte";
  import DeviceCard from "./DeviceCard.svelte";
  import {
    MessageSquare,
    Package,
    ShieldCheck,
    PanelLeftClose,
    PanelLeftOpen,
    Plus,
    FolderClosed,
    ChevronDown,
    ChevronRight,
    Search,
    Star,
    SettingsIcon,
    Pencil,
    Check,
    X,
  } from "./icons";

  export let view: "home" | "chat" | "models" | "agent" | "settings" | "pairing";
  export let onTab: (v: "home" | "models" | "agent") => void;
  export let onOpenSettings: () => void;
  export let conversations: Conversation[] = [];
  export let activeId: string | null = null;
  export let onSelect: (id: string) => void;
  export let onNew: () => void;
  export let onTogglePin: (id: string, pinned: boolean) => void;
  export let onOpenDownloads: () => void;
  export let onRename: (id: string, title: string) => void;
  export let reduceMotion = false;

  // Easter egg 2/3: click the logo mark 7 times quickly and it spins
  // once, with a one-line caption referencing the logo's own "motion"
  // design intent (see Logo.svelte's comment) -- a wink, not a feature.
  let logoClicks = 0;
  let logoClickTimer: ReturnType<typeof setTimeout> | null = null;
  let logoEgg = false;
  let logoEggTimer: ReturnType<typeof setTimeout> | null = null;
  let logoSpinning = false;

  function onLogoClick() {
    logoClicks++;
    if (logoClickTimer) clearTimeout(logoClickTimer);
    logoClickTimer = setTimeout(() => (logoClicks = 0), 1500);
    if (logoClicks >= 7) {
      logoClicks = 0;
      if (!reduceMotion) {
        logoSpinning = true;
        setTimeout(() => (logoSpinning = false), 700);
      }
      logoEgg = true;
      if (logoEggTimer) clearTimeout(logoEggTimer);
      logoEggTimer = setTimeout(() => (logoEgg = false), 3000);
    }
  }

  let renamingId: string | null = null;
  let renameDraft = "";

  function startRename(c: Conversation) {
    renamingId = c.ID;
    renameDraft = c.Title || c.Model || "";
  }

  function commitRename() {
    const title = renameDraft.trim();
    if (renamingId && title) onRename(renamingId, title);
    renamingId = null;
  }

  function cancelRename() {
    renamingId = null;
  }

  // Minimal Svelte action: focuses (and selects) an input the instant
  // it's mounted, since autofocus on the <input> itself doesn't run
  // reliably for an element that only exists conditionally behind
  // {#if renamingId === c.ID}.
  function focusOnMount(el: HTMLInputElement) {
    el.focus();
    el.select();
  }

  function onRenameKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      commitRename();
    } else if (e.key === "Escape") {
      e.preventDefault();
      cancelRename();
    }
  }

  let collapsed = false;
  let search = "";
  let projectsOpen = true;
  let projects: Project[] = [];

  onMount(async () => {
    // GET /api/runyx/projects returns a bare JSON `null` (nil Go slice)
    // when there are none yet, same as /api/runyx/conversations — guard
    // against that, not just a request error.
    projects = (await api.listProjects().catch(() => [])) ?? [];
  });

  async function addProject() {
    const name = window.prompt("Project name");
    if (!name) return;
    const p = await api.newProject(name).catch(() => null);
    if (p) projects = [...projects, p];
  }

  $: filtered = conversations.filter((c) =>
    (c.Title || c.Model || "").toLowerCase().includes(search.toLowerCase())
  );

  function dayBucket(iso: string): string {
    const d = new Date(iso);
    const now = new Date();
    const diffDays = Math.floor((now.setHours(0, 0, 0, 0) - new Date(d).setHours(0, 0, 0, 0)) / 86400000);
    if (diffDays <= 0) return "Today";
    if (diffDays === 1) return "Yesterday";
    if (diffDays < 7) return "This week";
    return "Earlier";
  }

  $: groups = (() => {
    const order = ["Today", "Yesterday", "This week", "Earlier"];
    const map: Record<string, Conversation[]> = {};
    for (const c of filtered) {
      const b = dayBucket(c.UpdatedAt || c.CreatedAt);
      (map[b] ??= []).push(c);
    }
    return order.filter((k) => map[k]?.length).map((k) => ({ label: k, items: map[k] }));
  })();
</script>

<nav class:collapsed>
  <div class="top">
    <button class="collapse-btn" on:click={() => (collapsed = !collapsed)} aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}>
      {#if collapsed}
        <PanelLeftOpen size={17} aria-hidden="true" />
      {:else}
        <span class="logo-click" class:spinning={logoSpinning} on:click|stopPropagation={onLogoClick} role="presentation">
          <Logo size={20} />
        </span>
        <span class="wordmark">Runyx</span>
        <PanelLeftClose size={16} aria-hidden="true" />
      {/if}
    </button>
    {#if logoEgg}
      <p class="logo-egg" transition:fade={{ duration: reduceMotion ? 0 : 160 }}>Fast by design. You found the proof.</p>
    {/if}
  </div>

  <div class="tabs">
    <button class="tab" class:active={view === "home" || view === "chat"} title="Chat" aria-label="Chat" on:click={() => onTab("home")}>
      <MessageSquare size={17} aria-hidden="true" />
      {#if !collapsed}<span>Chat</span>{/if}
    </button>
    <button class="tab" class:active={view === "models"} title="Models" aria-label="Models" on:click={() => onTab("models")}>
      <Package size={17} aria-hidden="true" />
      {#if !collapsed}<span>Models</span>{/if}
    </button>
    <button class="tab" class:active={view === "agent"} title="Agent" aria-label="Agent" on:click={() => onTab("agent")}>
      <ShieldCheck size={17} aria-hidden="true" />
      {#if !collapsed}<span>Agent</span>{/if}
    </button>
  </div>

  {#if !collapsed}
    <div class="quick-links">
      <button class="quick-link" on:click={onNew}>
        <Plus size={15} aria-hidden="true" />
        <span>New chat</span>
      </button>
      <button class="quick-link" on:click={onOpenSettings}>
        <SettingsIcon size={15} aria-hidden="true" />
        <span>Settings</span>
      </button>
    </div>

    <div class="section">
      <button class="section-header" on:click={() => (projectsOpen = !projectsOpen)}>
        {#if projectsOpen}<ChevronDown size={13} aria-hidden="true" />{:else}<ChevronRight size={13} aria-hidden="true" />{/if}
        <FolderClosed size={14} aria-hidden="true" />
        <span>Projects</span>
        <button class="add-inline" on:click|stopPropagation={addProject} aria-label="New project">
          <Plus size={13} aria-hidden="true" />
        </button>
      </button>
      {#if projectsOpen}
        <ul class="project-list">
          {#each projects as p}
            <li>{p.Name}</li>
          {/each}
          {#if projects.length === 0}
            <li class="empty">No projects yet.</li>
          {/if}
        </ul>
      {/if}
    </div>

    <div class="section recents">
      <div class="search-box">
        <Search size={13} aria-hidden="true" />
        <input placeholder="Search conversations" bind:value={search} />
      </div>
      <h2>Recents</h2>
      {#if filtered.length === 0}
        <p class="empty">No conversations yet.</p>
      {/if}
      {#each groups as g}
        <h3>{g.label}</h3>
        <ul>
          {#each g.items as c}
            <li class:active={c.ID === activeId}>
              {#if renamingId === c.ID}
                <input
                  class="rename-input"
                  bind:value={renameDraft}
                  on:keydown={onRenameKeydown}
                  on:blur={commitRename}
                  use:focusOnMount
                />
                <button class="pin" title="Save" aria-label="Save title" on:mousedown|preventDefault={commitRename}>
                  <Check size={13} aria-hidden="true" />
                </button>
                <button class="pin" title="Cancel" aria-label="Cancel rename" on:mousedown|preventDefault={cancelRename}>
                  <X size={13} aria-hidden="true" />
                </button>
              {:else}
                <button class="title" on:click={() => onSelect(c.ID)} on:dblclick={() => startRename(c)}>
                  {c.Title || c.Model || "…"}
                </button>
                <button class="pin rename" title="Rename" aria-label="Rename conversation" on:click={() => startRename(c)}>
                  <Pencil size={12} aria-hidden="true" />
                </button>
                <button class="pin" title={c.Pinned ? "Unpin" : "Pin"} on:click={() => onTogglePin(c.ID, !c.Pinned)}>
                  <Star size={13} fill={c.Pinned ? "currentColor" : "none"} aria-hidden="true" />
                </button>
              {/if}
            </li>
          {/each}
        </ul>
      {/each}
    </div>
  {/if}

  <DeviceCard {collapsed} {onOpenDownloads} />
</nav>

<style>
  nav {
    width: 280px;
    flex-shrink: 0;
    background: var(--bg-sunken);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    transition: width 0.15s ease;
  }
  nav.collapsed {
    width: 60px;
  }
  .top {
    padding: 0.75rem 0.6rem 0.25rem;
  }
  .collapse-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    border: none;
    background: transparent;
    padding: 0.4rem;
    border-radius: var(--radius-md);
    color: var(--text);
  }
  .wordmark {
    flex: 1;
    text-align: left;
    font-weight: 600;
  }
  .logo-click {
    display: inline-flex;
    border-radius: var(--radius-sm);
  }
  .logo-click.spinning {
    animation: runyx-spin-y 0.7s var(--ease-fluid);
  }
  @keyframes runyx-spin-y {
    from {
      transform: rotate(0deg) scale(1);
    }
    50% {
      transform: rotate(180deg) scale(1.15);
    }
    to {
      transform: rotate(360deg) scale(1);
    }
  }
  .logo-egg {
    margin: 0;
    padding: 0.15rem 0.75rem 0.4rem;
    font-size: 0.72rem;
    color: var(--text-muted);
    font-style: italic;
  }
  .tabs {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.3rem 0.5rem;
  }
  .tab {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    border: none;
    background: transparent;
    border-radius: var(--radius-md);
    padding: 0.5rem 0.6rem;
    color: var(--text-muted);
    font-size: 0.88rem;
  }
  .tab.active {
    background: var(--bg-elevated);
    color: var(--text);
  }
  .quick-links {
    padding: 0.4rem 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .quick-link {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    border: none;
    background: transparent;
    border-radius: var(--radius-md);
    padding: 0.45rem 0.6rem;
    color: var(--text-muted);
    font-size: 0.85rem;
  }
  .quick-link:hover {
    color: var(--text);
  }
  .section {
    padding: 0.3rem 0.5rem;
  }
  .section-header {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    width: 100%;
    border: none;
    background: transparent;
    padding: 0.4rem 0.3rem;
    color: var(--text-muted);
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .add-inline {
    margin-left: auto;
    border: none;
    background: transparent;
    color: var(--text-muted);
    padding: 0.1rem;
  }
  .project-list {
    list-style: none;
    margin: 0;
    padding: 0 0.3rem;
    font-size: 0.84rem;
    color: var(--text);
  }
  .project-list li {
    padding: 0.3rem 0;
  }
  .recents {
    flex: 1;
    overflow-y: auto;
  }
  .search-box {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 0.4rem 0.6rem;
    margin: 0.2rem 0.3rem 0.5rem;
    color: var(--text-muted);
  }
  .search-box input {
    border: none;
    background: transparent;
    outline: none;
    color: var(--text);
    font-size: 0.82rem;
    width: 100%;
  }
  h2 {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    margin: 0.4rem 0.3rem 0.1rem;
    font-weight: 500;
  }
  h3 {
    font-size: 0.68rem;
    text-transform: uppercase;
    color: var(--text-muted);
    margin: 0.6rem 0.3rem 0.1rem;
    font-weight: 500;
  }
  .empty {
    color: var(--text-muted);
    font-size: 0.82rem;
    padding: 0 0.3rem;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    align-items: center;
    border-radius: var(--radius-sm);
  }
  li.active {
    background: var(--bg-elevated);
  }
  li .title {
    flex: 1;
    text-align: left;
    background: none;
    border: none;
    padding: 0.5rem 0.6rem;
    color: var(--text);
    font-size: 0.86rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  li .pin {
    background: none;
    border: none;
    color: var(--text-muted);
    padding: 0.4rem;
    flex: 0 0 auto;
  }
  li .rename {
    opacity: 0;
  }
  li:hover .rename {
    opacity: 1;
  }
  .rename-input {
    flex: 1;
    min-width: 0;
    background: var(--bg-elevated);
    border: 1px solid var(--accent);
    border-radius: var(--radius-sm);
    padding: 0.4rem 0.5rem;
    margin: 0.1rem 0.2rem;
    color: var(--text);
    font-size: 0.86rem;
    outline: none;
  }
  @media (max-width: 760px) {
    nav {
      display: none;
    }
  }
</style>
