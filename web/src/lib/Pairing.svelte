<script lang="ts">
  import { api } from "./api";
  import { t, type Lang } from "./i18n";

  export let lang: Lang;

  let qrUrl: string | null = null;
  let error: string | null = null;
  let busy = false;

  async function generate() {
    busy = true;
    error = null;
    qrUrl = null;
    const res = await api.pairingQRCode("android");
    if (res.ok) {
      qrUrl = res.url;
    } else {
      error = res.error;
    }
    busy = false;
  }
</script>

<div class="page">
  <h1>{t(lang, "pairingTitle")}</h1>
  <p class="desc">{t(lang, "pairingDesc")}</p>

  <button on:click={generate} disabled={busy}>{t(lang, "pairingGenerate")}</button>

  {#if qrUrl}
    <img src={qrUrl} alt="pairing QR code" class="qr" />
  {/if}
  {#if error}
    <p class="error">{error}</p>
  {/if}
</div>

<style>
  .page {
    flex: 1;
    overflow-y: auto;
    padding: 2rem 2.5rem;
    max-width: 480px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 1rem;
  }
  h1 {
    margin: 0;
  }
  .desc {
    color: var(--text-muted);
    margin: 0;
  }
  button {
    padding: 0.6rem 1.1rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .qr {
    width: 256px;
    height: 256px;
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
    background: #fff;
    padding: 0.75rem;
  }
  .error {
    color: var(--danger, #c0392b);
    max-width: 440px;
  }
</style>
