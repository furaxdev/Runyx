<script lang="ts">
  import { fly } from "svelte/transition";

  export let message = "";
  export let reduceMotion = false;

  $: duration = reduceMotion ? 0 : 220;
</script>

{#if message}
  <div class="toast" role="status" transition:fly={{ y: 20, duration }}>
    {message}
  </div>
{/if}

<style>
  .toast {
    position: fixed;
    left: 50%;
    bottom: 2rem;
    transform: translateX(-50%);
    max-width: min(420px, 90vw);
    background: var(--bg-elevated);
    border: 1px solid var(--accent);
    color: var(--text);
    border-radius: var(--radius-lg);
    padding: 0.7rem 1.1rem;
    box-shadow: var(--shadow);
    font-size: 0.85rem;
    text-align: center;
    z-index: 200;
  }
</style>
