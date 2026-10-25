import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// The build output here is embedded into the runyx Go binary via
// go:embed (see cmd/runyx and internal/daemon, wired up in M2) and
// served at "/" by the daemon's HTTP server, alongside /api and /v1.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:11535",
      "/v1": "http://127.0.0.1:11535",
    },
  },
});
