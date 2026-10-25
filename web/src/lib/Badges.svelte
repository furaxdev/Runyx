<script lang="ts">
  import { Eye, Wrench, Brain, Code, Globe, AlignLeft } from "./icons";

  export let capabilities: string[] = [];
  export let compact = false;

  const meta: Record<string, { label: string; icon: any }> = {
    vision: { label: "Vision", icon: Eye },
    tools: { label: "Tools", icon: Wrench },
    reasoning: { label: "Reasoning", icon: Brain },
    code: { label: "Code", icon: Code },
    multilingual: { label: "Multilingual", icon: Globe },
    long_context: { label: "Long context", icon: AlignLeft },
  };
</script>

<div class="badges" class:compact>
  {#each capabilities as cap}
    {#if meta[cap]}
      <span class="badge" title={meta[cap].label}>
        <svelte:component this={meta[cap].icon} size={compact ? 11 : 13} aria-hidden="true" />
        {#if !compact}
          {meta[cap].label}
        {/if}
      </span>
    {/if}
  {/each}
</div>

<style>
  .badges {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.72rem;
    color: var(--text-muted);
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.15rem 0.5rem;
  }
  .badges.compact .badge {
    padding: 0.15rem 0.3rem;
  }
</style>
