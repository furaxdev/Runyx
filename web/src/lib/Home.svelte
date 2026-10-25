<script lang="ts">
  import { createEventDispatcher, onMount } from "svelte";
  import { api, type RegistryModel } from "./api";
  import Composer from "./Composer.svelte";
  import { PenLine, BookOpen, Code, FileText } from "./icons";

  export let installedModels: RegistryModel[] = [];
  export let model = "";
  export let onBrowseMore: () => void;
  export let reduceMotion = false;

  const dispatch = createEventDispatcher<{
    send: { text: string; attachments: { file_name: string; kind: string; text: string }[] };
  }>();

  let firstName = "";

  onMount(async () => {
    const who = await api.whoami().catch(() => ({ first_name: "" }));
    firstName = who.first_name;
  });

  function greeting(): string {
    const h = new Date().getHours();
    const time = h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening";
    return firstName ? `${time}, ${firstName}` : time;
  }

  const shortcuts = [
    { label: "Write", icon: PenLine, prompt: "Help me write " },
    { label: "Learn", icon: BookOpen, prompt: "Explain " },
    { label: "Code", icon: Code, prompt: "Write code to " },
    { label: "Analyze a file", icon: FileText, prompt: "" },
  ];

  function onSend(e: CustomEvent<{ text: string; attachments: any[] }>) {
    dispatch("send", e.detail);
  }
</script>

<div class="home">
  <h1>{greeting()}</h1>

  {#if installedModels.length === 0}
    <p class="invite">
      No models installed yet. Pick one to get started:
    </p>
    <div class="suggestions">
      <button class="suggestion-card" on:click={onBrowseMore}>Browse models</button>
    </div>
  {:else}
    <div class="composer-wrap">
      <Composer {installedModels} bind:model {onBrowseMore} autofocus on:send={onSend} {reduceMotion} />
    </div>

    <div class="shortcuts">
      {#each shortcuts as s}
        <button class="shortcut" on:click={() => dispatch("send", { text: s.prompt, attachments: [] })}>
          <svelte:component this={s.icon} size={15} aria-hidden="true" />
          {s.label}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .home {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.5rem;
    padding: 2rem;
    text-align: center;
  }
  h1 {
    font-size: 2.25rem;
    margin: 0;
    color: var(--text);
  }
  .invite {
    color: var(--text-muted);
    margin: 0;
  }
  .suggestions {
    display: flex;
    gap: 0.75rem;
  }
  .suggestion-card {
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    border-radius: var(--radius-md);
    padding: 0.6rem 1.2rem;
    color: var(--text);
  }
  .composer-wrap {
    width: 100%;
    max-width: 720px;
  }
  .shortcuts {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 0.6rem;
  }
  .shortcut {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    border: 1px solid var(--border);
    background: var(--bg-elevated);
    border-radius: 999px;
    padding: 0.5rem 1rem;
    color: var(--text-muted);
    font-size: 0.85rem;
  }
  .shortcut:hover {
    color: var(--text);
  }
</style>
