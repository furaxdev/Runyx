<script lang="ts">
  import { createEventDispatcher, tick } from "svelte";
  import { fade, scale } from "svelte/transition";
  import { api, type RegistryModel } from "./api";
  import { Plus, Mic, Square, SendHorizontal } from "./icons";
  import ModelSelect from "./ModelSelect.svelte";

  export let installedModels: RegistryModel[] = [];
  export let model = "";
  export let onBrowseMore: () => void;
  export let disabled = false;
  export let placeholder = "Type a message...";
  export let autofocus = false;
  export let reduceMotion = false;

  $: dropFadeDuration = reduceMotion ? 0 : 140;

  const dispatch = createEventDispatcher<{
    send: { text: string; attachments: { file_name: string; kind: string; text: string }[] };
  }>();

  // Grows the textarea from a 2-line rest height up to MAX_LINES, then
  // scrolls — never grows the whole page past that.
  const MAX_LINES = 8;

  let input = "";
  let textareaEl: HTMLTextAreaElement;
  let fileInput: HTMLInputElement;
  let pendingAttachments: { file_name: string; kind: string; text: string }[] = [];
  let recording = false;
  let mediaRecorder: MediaRecorder | null = null;
  let recordedChunks: Blob[] = [];
  let dragging = false;
  // Counts nested dragenter/dragleave pairs (the browser fires these on
  // every child element the pointer crosses) so the drop highlight
  // doesn't flicker off while still dragging over a child node.
  let dragDepth = 0;

  $: hasContent = input.trim().length > 0 || pendingAttachments.length > 0;
  $: wordCount = input.trim().length === 0 ? 0 : input.trim().split(/\s+/).length;

  function autoGrow() {
    if (!textareaEl) return;
    textareaEl.style.height = "auto";
    const lineHeight = parseFloat(getComputedStyle(textareaEl).lineHeight) || 20;
    const maxHeight = lineHeight * MAX_LINES;
    textareaEl.style.height = `${Math.min(textareaEl.scrollHeight, maxHeight)}px`;
  }

  async function addFiles(files: Iterable<File>) {
    for (const f of files) {
      try {
        const res = await api.uploadAttachment(f);
        pendingAttachments = [...pendingAttachments, res];
      } catch (err) {
        alert(`${f.name}: ${err}`);
      }
    }
  }

  async function onFilePicked(e: Event) {
    const files = (e.target as HTMLInputElement).files;
    if (!files) return;
    await addFiles(files);
    fileInput.value = "";
  }

  function onDragEnter(e: DragEvent) {
    if (!e.dataTransfer?.types.includes("Files")) return;
    e.preventDefault();
    dragDepth++;
    dragging = true;
  }

  function onDragOver(e: DragEvent) {
    if (!e.dataTransfer?.types.includes("Files")) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
  }

  function onDragLeave(e: DragEvent) {
    if (!e.dataTransfer?.types.includes("Files")) return;
    e.preventDefault();
    dragDepth = Math.max(0, dragDepth - 1);
    if (dragDepth === 0) dragging = false;
  }

  async function onDrop(e: DragEvent) {
    e.preventDefault();
    dragDepth = 0;
    dragging = false;
    const files = e.dataTransfer?.files;
    if (!files || files.length === 0) return;
    await addFiles(files);
  }

  async function toggleRecording() {
    if (recording) {
      mediaRecorder?.stop();
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      recordedChunks = [];
      mediaRecorder = new MediaRecorder(stream);
      mediaRecorder.ondataavailable = (e) => recordedChunks.push(e.data);
      mediaRecorder.onstop = async () => {
        recording = false;
        stream.getTracks().forEach((t) => t.stop());
        const blob = new Blob(recordedChunks, { type: "audio/webm" });
        try {
          const result = await api.transcribeAudio(blob);
          input = (input ? input + " " : "") + result.text;
          await tick();
          autoGrow();
        } catch (err) {
          alert(`Transcription: ${err}`);
        }
      };
      mediaRecorder.start();
      recording = true;
    } catch (err) {
      alert(`Microphone: ${err}`);
    }
  }

  async function send() {
    const text = input.trim();
    if (!text && pendingAttachments.length === 0) return;
    dispatch("send", { text, attachments: pendingAttachments });
    input = "";
    pendingAttachments = [];
    await tick();
    autoGrow();
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }
</script>

<div
  class="composer"
  class:dragging
  role="group"
  aria-label="Message composer, drop a file here to attach it"
  on:dragenter={onDragEnter}
  on:dragover={onDragOver}
  on:dragleave={onDragLeave}
  on:drop={onDrop}
>
  {#if dragging}
    <div class="drop-overlay" transition:fade={{ duration: dropFadeDuration }}>
      <span transition:scale={{ duration: dropFadeDuration, start: 0.9 }}>Drop to attach</span>
    </div>
  {/if}
  {#if pendingAttachments.length > 0}
    <div class="chips">
      {#each pendingAttachments as a (a.file_name)}
        <span class="chip" title={a.text.slice(0, 300)} transition:scale={{ duration: dropFadeDuration, start: 0.85 }}
          >{a.file_name} ({a.kind})</span
        >
      {/each}
    </div>
  {/if}

  <textarea
    bind:this={textareaEl}
    bind:value={input}
    {placeholder}
    on:keydown={onKeydown}
    on:input={autoGrow}
    rows="2"
    {autofocus}
  />

  <div class="toolbar">
    <button type="button" class="round-btn" title="Attach a file" aria-label="Attach a file" on:click={() => fileInput.click()}>
      <Plus size={18} aria-hidden="true" />
    </button>
    <input type="file" bind:this={fileInput} on:change={onFilePicked} style="display: none" multiple />

    <div class="model-select-wrap">
      <ModelSelect installed={installedModels} selected={model} {onBrowseMore} on:select={(e) => (model = e.detail)} />
    </div>

    <div class="spacer" />

    {#if input.length > 0}
      <span class="counter" transition:fade={{ duration: dropFadeDuration }}>
        {wordCount}
        {wordCount === 1 ? "word" : "words"} &middot; {input.length}
      </span>
    {/if}

    <button
      type="button"
      class="round-btn"
      class:active={recording}
      title={recording ? "Recording... click to stop" : "Talk (push-to-talk)"}
      aria-label={recording ? "Stop recording" : "Start voice input"}
      on:click={toggleRecording}
    >
      {#if recording}
        <Square size={16} aria-hidden="true" />
      {:else}
        <Mic size={18} aria-hidden="true" />
      {/if}
    </button>

    <button
      type="button"
      class="round-btn send"
      class:filled={hasContent}
      on:click={send}
      disabled={disabled || !hasContent}
      aria-label="Send message"
    >
      <SendHorizontal size={18} aria-hidden="true" />
    </button>
  </div>
</div>

<style>
  .composer {
    position: relative;
    max-width: 720px;
    width: 100%;
    margin: 0 auto;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 24px;
    padding: 0.55rem 0.6rem 0.5rem;
    box-shadow: var(--shadow);
    transition: box-shadow var(--transition-fast), border-color var(--transition-fast);
  }
  .composer:focus-within {
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 35%, transparent), var(--shadow);
  }
  .composer.dragging {
    border-color: var(--accent);
    border-style: dashed;
  }
  .drop-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 24px;
    background: color-mix(in srgb, var(--accent) 10%, var(--bg-elevated) 90%);
    color: var(--accent);
    font-size: 0.9rem;
    font-weight: 600;
    pointer-events: none;
    z-index: 1;
  }
  .chips {
    display: flex;
    gap: 0.5rem;
    padding: 0.1rem 0.5rem 0.5rem;
    flex-wrap: wrap;
  }
  .chip {
    font-size: 0.78rem;
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.25rem 0.7rem;
    color: var(--text-muted);
  }
  textarea {
    display: block;
    width: 100%;
    min-height: 2.8em;
    max-height: calc(1.4em * 8);
    border: none;
    background: transparent;
    resize: none;
    font-family: inherit;
    font-size: 0.95rem;
    line-height: 1.4;
    color: var(--text);
    outline: none;
    padding: 0.35rem 0.5rem 0.3rem;
    overflow-y: auto;
  }
  textarea::placeholder {
    color: var(--text-muted);
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.1rem 0.15rem 0;
  }
  .spacer {
    flex: 1;
  }
  .counter {
    font-size: 0.72rem;
    color: var(--text-muted);
    white-space: nowrap;
    margin-right: 0.3rem;
    flex: 0 0 auto;
  }
  @media (max-width: 640px) {
    .counter {
      display: none;
    }
  }
  .model-select-wrap {
    min-width: 0;
    flex: 0 1 auto;
    overflow: hidden;
  }
  .round-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.1rem;
    height: 2.1rem;
    flex: 0 0 auto;
    border: none;
    border-radius: 999px;
    background: transparent;
    color: var(--text-muted);
    transition: background var(--transition-fast), color var(--transition-fast);
  }
  .round-btn:hover {
    background: var(--bg-sunken);
  }
  .round-btn.active {
    color: var(--accent-contrast);
    background: var(--accent);
  }
  .round-btn.send {
    color: var(--text-muted);
    background: var(--bg-sunken);
  }
  .round-btn.send.filled {
    color: var(--accent-contrast);
    background: var(--accent);
  }
  .round-btn.send:disabled {
    opacity: 0.55;
    cursor: default;
  }

  @media (max-width: 640px) {
    .composer {
      margin: 0 12px;
      width: calc(100% - 24px);
      max-width: none;
    }
    .round-btn {
      width: 44px;
      height: 44px;
    }
  }
</style>
