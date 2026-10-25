# Architecture

Runyx is a single Go binary (`runyx`) acting as CLI, daemon, HTTP server and
embedded-UI host. It never reimplements LLM inference: it manages `llama.cpp`
(`llama-server`) as a supervised subprocess and adds everything around it
that makes local LLMs pleasant to use.

## Process model

```
runyx (CLI)  --IPC/HTTP-->  runyxd (daemon, :11535 by default)
                                 |
                                 +-- backend manager
                                 |      +-- llama-server (model A), subprocess
                                 |      +-- llama-server (model B), subprocess
                                 |
                                 +-- HTTP API
                                 |      +-- /api/*  (Ollama-compatible)
                                 |      +-- /v1/*   (OpenAI-compatible)
                                 |      +-- /ui/*   (embedded web UI, go:embed)
                                 |
                                 +-- SQLite store (config, history, models,
                                     tuning cache)
```

The CLI and daemon are the same binary; `runyx serve` runs the daemon in the
foreground, and `runyx` subcommands talk to it over localhost HTTP, starting
it on demand if it is not already running (like a client/server pattern,
similar in spirit to Ollama's but independently implemented from its public
API docs only — no source was copied).

## Packages (`internal/`)

- `config` — on-disk config (TOML), model directory resolution, per-model
  tuning cache.
- `hwdetect` — CPU/RAM/GPU/VRAM detection (`doctor`, `recommend`, `tune`).
- `downloader` — resumable, parallel, SHA256-verified downloads from Hugging
  Face, with disk-space checks.
- `modelstore` — local model registry (GGUF files + metadata), import of
  local GGUFs and of Ollama's existing blob store.
- `backend` — llama.cpp build fetch/update, process supervision per loaded
  model (start/stop/health/keep-alive/LRU eviction).
- `daemon` — wires everything together, owns the SQLite DB and the HTTP
  server lifecycle.
- `api` — HTTP handlers: Ollama-compatible (`/api/...`) and OpenAI-compatible
  (`/v1/...`), translating to the per-model llama-server instances.
- `version` — build-time version info.

## Why Go + llama.cpp subprocess (not cgo bindings)

Running `llama-server` as a subprocess (rather than linking llama.cpp via
cgo) keeps the core binary small, lets each backend build (CPU/CUDA/Vulkan/
Metal/ROCm) be updated independently of `runyx` itself, and avoids pulling a
C++ toolchain into the Go build. The cost is one local HTTP hop per model,
which is negligible next to generation time and is the target of the
"< 50 ms first-token overhead" budget in the roadmap.

## Data layout

- Config: OS-standard config dir (`~/.config/runyx` on Linux, etc.), file
  `config.toml`.
- Models: configurable directory, default `<data dir>/runyx/models`, GGUF
  files named by content hash + metadata sidecar.
- State DB: `<data dir>/runyx/runyx.db` (SQLite): history, projects, model
  registry, tuning cache.
- Backends: `<data dir>/runyx/backends/<platform>-<variant>/` holding the
  downloaded `llama-server` build(s).

## UI

`web/` is a Svelte + Vite app. Its production build is embedded into the
Go binary via `go:embed` and served at `/` by the daemon. The desktop shell
(Wails or equivalent) and the PWA both point at the same embedded UI, so
there is exactly one UI codebase for web, desktop and (eventually) the
in-app views Android does not share.
