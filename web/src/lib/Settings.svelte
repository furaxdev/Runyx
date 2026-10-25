<script lang="ts">
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
</style>
