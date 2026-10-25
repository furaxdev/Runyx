<script lang="ts">
  import { onMount } from "svelte";
  import { fade } from "svelte/transition";
  import "./lib/theme.css";
  import { api, type Conversation, type Message, type RegistryModel } from "./lib/api";
  import { detectLang, t, type Lang } from "./lib/i18n";
  import Sidebar from "./lib/Sidebar.svelte";
  import Home from "./lib/Home.svelte";
  import Chat from "./lib/Chat.svelte";
  import CommandPalette from "./lib/CommandPalette.svelte";
  import Models from "./lib/Models.svelte";
  import Pairing from "./lib/Pairing.svelte";
  import Settings from "./lib/Settings.svelte";
  import Agent from "./lib/Agent.svelte";
  import SearchMessages from "./lib/SearchMessages.svelte";
  import ShortcutsHelp from "./lib/ShortcutsHelp.svelte";
  import EasterEggToast from "./lib/EasterEggToast.svelte";

  type View = "home" | "chat" | "models" | "pairing" | "settings" | "agent";
  let view: View = "home";

  let lang: Lang = detectLang();
  let theme: "system" | "light" | "dark" = (safeGet("runyx.theme") as any) ?? "system";
  let reduceMotion = safeGet("runyx.reduceMotion") === "1";

  let conversations: Conversation[] = [];
  let activeId: string | null = null;
  let messages: Message[] = [];
  let registryModels: RegistryModel[] = [];
  let currentModel = "";
  let paletteOpen = false;
  let searchOpen = false;
  let shortcutsOpen = false;

  // Keeps the three overlays mutually exclusive -- opening one closes
  // whichever other was already open, rather than letting them stack
  // (the Shortcuts modal has no visible scrim gap for Search to sit
  // behind, so a stray Ctrl+Shift+F while Ctrl+/ was still up would
  // otherwise leave two overlays showing at once).
  function openOverlay(which: "palette" | "search" | "shortcuts" | null) {
    paletteOpen = which === "palette";
    searchOpen = which === "search";
    shortcutsOpen = which === "shortcuts";
  }
  let eggMessage = "";
  let eggTimer: ReturnType<typeof setTimeout> | null = null;

  function showEgg(message: string) {
    eggMessage = message;
    if (eggTimer) clearTimeout(eggTimer);
    eggTimer = setTimeout(() => (eggMessage = ""), 4000);
  }

  function safeGet(k: string): string | null {
    try {
      return localStorage.getItem(k);
    } catch {
      return null;
    }
  }
  function safeSet(k: string, v: string) {
    try {
      localStorage.setItem(k, v);
    } catch {
      /* ignore (private window, blocked storage, etc.) */
    }
  }

  function applyTheme() {
    document.documentElement.setAttribute("data-theme", theme === "system" ? "" : theme);
    document.body.classList.toggle("reduce-motion", reduceMotion);
  }

  async function refreshConversations() {
    conversations = (await api.listConversations().catch(() => [])) ?? [];
  }

  async function refreshMessages() {
    if (!activeId) return;
    messages = (await api.messages(activeId).catch(() => [])) ?? [];
  }

  async function refreshRegistry() {
    registryModels = await api.registry().catch(() => []);
  }

  $: installedModels = registryModels.filter((m) => m.installed);

  async function selectConversation(id: string) {
    activeId = id;
    view = "chat";
    pendingFirstText = "";
    pendingFirstAttachments = [];
    const c = conversations.find((c) => c.ID === id);
    if (c) currentModel = c.Model;
    await refreshMessages();
  }

  async function newConversation() {
    const c = await api.newConversation("", currentModel || installedModels[0]?.pull_spec || "");
    await refreshConversations();
    await selectConversation(c.ID);
  }

  // Home's composer has no conversation yet: create one, then hand the
  // typed text to the freshly-mounted Chat (via initialText), which
  // sends it with its own normal streaming logic.
  let pendingFirstText = "";
  let pendingFirstAttachments: { file_name: string; kind: string; text: string }[] = [];

  async function onHomeSend(e: CustomEvent<{ text: string; attachments: { file_name: string; kind: string; text: string }[] }>) {
    const c = await api.newConversation("", currentModel || installedModels[0]?.pull_spec || "");
    await refreshConversations();
    pendingFirstText = e.detail.text;
    pendingFirstAttachments = e.detail.attachments;
    activeId = c.ID;
    view = "chat";
    await refreshMessages();
  }

  async function togglePin(id: string, pinned: boolean) {
    await api.pin(id, pinned);
    await refreshConversations();
  }

  async function renameConversation(id: string, title: string) {
    await api.rename(id, title).catch((err) => alert(`Rename failed: ${err}`));
    await refreshConversations();
  }

  function onTab(v: "home" | "models" | "agent") {
    view = v;
    if (v === "home" && activeId) view = "chat";
  }

  async function onSearchJump(conversationId: string) {
    await selectConversation(conversationId);
  }

  // Easter egg 1/3: the Konami code. A harmless nod for anyone who
  // still has it in their fingers -- no gameplay effect, just a toast
  // and (motion allowed) a brief accent-colored pulse around the app.
  const KONAMI = ["ArrowUp", "ArrowUp", "ArrowDown", "ArrowDown", "ArrowLeft", "ArrowRight", "ArrowLeft", "ArrowRight", "b", "a"];
  let konamiProgress = 0;
  let konamiPulse = false;

  function checkKonami(key: string) {
    const want = KONAMI[konamiProgress];
    if (key.toLowerCase() === want?.toLowerCase()) {
      konamiProgress++;
      if (konamiProgress === KONAMI.length) {
        konamiProgress = 0;
        showEgg("Secret unlocked. You have excellent taste in keyboard shortcuts.");
        if (!reduceMotion) {
          konamiPulse = true;
          setTimeout(() => (konamiPulse = false), 900);
        }
      }
    } else {
      konamiProgress = key.toLowerCase() === KONAMI[0]?.toLowerCase() ? 1 : 0;
    }
  }

  onMount(async () => {
    applyTheme();
    await refreshConversations();
    await refreshRegistry();
    if (!currentModel) currentModel = installedModels[0]?.pull_spec ?? "";
    setInterval(refreshRegistry, 10000);

    window.addEventListener("keydown", (e) => {
      const mod = e.ctrlKey || e.metaKey;
      if (mod && e.key.toLowerCase() === "k") {
        e.preventDefault();
        openOverlay("palette");
        return;
      }
      if (mod && e.shiftKey && e.key.toLowerCase() === "f") {
        e.preventDefault();
        openOverlay("search");
        return;
      }
      if (mod && e.shiftKey && e.key.toLowerCase() === "o") {
        e.preventDefault();
        newConversation();
        return;
      }
      if (mod && e.key === "/") {
        e.preventDefault();
        openOverlay("shortcuts");
        return;
      }
      // Konami code only fires outside text inputs -- otherwise typing
      // the letters "b"/"a" while composing a message would eat input.
      const target = e.target as HTMLElement | null;
      const typing = target?.tagName === "INPUT" || target?.tagName === "TEXTAREA" || target?.isContentEditable;
      if (!typing && !mod) checkKonami(e.key);
    });

    // Without this, dropping a file anywhere outside the Composer's own
    // drop zone falls through to the browser's default behavior --
    // navigating the whole tab to the dropped file -- which would wipe
    // out the app. The Composer's own dragover/drop handlers still run
    // first (they stop propagation isn't needed since both just call
    // preventDefault), so a drop *on* the composer still attaches it.
    window.addEventListener("dragover", (e) => e.preventDefault());
    window.addEventListener("drop", (e) => e.preventDefault());
  });

  $: commands = [
    { label: t(lang, "newChat"), run: newConversation },
    { label: "Search messages", run: () => openOverlay("search") },
    { label: "Keyboard shortcuts", run: () => openOverlay("shortcuts") },
    { label: `${t(lang, "theme")}: ${t(lang, "light")}`, run: () => setTheme("light") },
    { label: `${t(lang, "theme")}: ${t(lang, "dark")}`, run: () => setTheme("dark") },
    { label: `${t(lang, "theme")}: ${t(lang, "system")}`, run: () => setTheme("system") },
  ];

  function setTheme(v: "system" | "light" | "dark") {
    theme = v;
    safeSet("runyx.theme", v);
    applyTheme();
  }

  function setReduceMotion(v: boolean) {
    reduceMotion = v;
    safeSet("runyx.reduceMotion", v ? "1" : "0");
    applyTheme();
  }

  $: viewFadeDuration = reduceMotion ? 0 : 160;
</script>

<div class="app" class:konami-pulse={konamiPulse}>
  <Sidebar
    {view}
    {onTab}
    onOpenSettings={() => (view = "settings")}
    {conversations}
    {activeId}
    onSelect={selectConversation}
    onNew={() => {
      activeId = null;
      view = "home";
    }}
    onTogglePin={togglePin}
    onOpenDownloads={() => (view = "models")}
    onRename={renameConversation}
    {reduceMotion}
  />

  <main>
    {#if view === "home"}
      <div class="view-fade" in:fade={{ duration: viewFadeDuration }}>
        <Home {installedModels} bind:model={currentModel} onBrowseMore={() => (view = "models")} on:send={onHomeSend} {reduceMotion} />
      </div>
    {:else if view === "chat" && activeId}
      {#key activeId}
        <div class="view-fade" in:fade={{ duration: viewFadeDuration }}>
          <Chat
            conversationId={activeId}
            conversationTitle={conversations.find((c) => c.ID === activeId)?.Title ?? ""}
            model={currentModel}
            {messages}
            onMessagesChanged={refreshMessages}
            {installedModels}
            onBrowseMore={() => (view = "models")}
            initialText={pendingFirstText}
            initialAttachments={pendingFirstAttachments}
            {reduceMotion}
          />
        </div>
      {/key}
    {:else if view === "models"}
      <div class="view-fade" in:fade={{ duration: viewFadeDuration }}>
        <Models
          onUse={(spec) => {
            currentModel = spec;
            view = activeId ? "chat" : "home";
          }}
        />
      </div>
    {:else if view === "pairing"}
      <div class="view-fade" in:fade={{ duration: viewFadeDuration }}>
        <Pairing {lang} />
      </div>
    {:else if view === "settings"}
      <div class="view-fade" in:fade={{ duration: viewFadeDuration }}>
        <Settings {theme} {reduceMotion} onSetTheme={setTheme} onToggleReduceMotion={setReduceMotion} />
      </div>
    {:else if view === "agent"}
      <div class="view-fade" in:fade={{ duration: viewFadeDuration }}>
        <Agent onPairDevice={() => (view = "pairing")} />
      </div>
    {/if}
  </main>
</div>

<CommandPalette {lang} bind:open={paletteOpen} {commands} {reduceMotion} />
<SearchMessages bind:open={searchOpen} {conversations} onJump={onSearchJump} {reduceMotion} />
<ShortcutsHelp bind:open={shortcutsOpen} {reduceMotion} />
<EasterEggToast message={eggMessage} {reduceMotion} />

<style>
  .app {
    display: flex;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
    box-shadow: inset 0 0 0 0 var(--accent);
  }
  .app.konami-pulse {
    animation: runyx-konami-pulse 0.9s var(--ease-fluid);
  }
  @keyframes runyx-konami-pulse {
    0% {
      box-shadow: inset 0 0 0 0 var(--accent);
    }
    30% {
      box-shadow: inset 0 0 0 4px var(--accent);
    }
    100% {
      box-shadow: inset 0 0 0 0 var(--accent);
    }
  }
  main {
    flex: 1;
    display: flex;
    min-width: 0;
  }
  .view-fade {
    flex: 1;
    display: flex;
    min-width: 0;
    min-height: 0;
  }
</style>
