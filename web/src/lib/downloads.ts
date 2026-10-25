// Tiny shared store of in-progress model downloads, so the sidebar's
// device card can show a live count/button regardless of which page
// started the download (Models.svelte writes to this; DeviceCard.svelte
// reads it).
import { writable } from "svelte/store";

export interface ActiveDownload {
  id: string;
  name: string;
  downloaded: number;
  total: number;
}

export const activeDownloads = writable<Record<string, ActiveDownload>>({});
