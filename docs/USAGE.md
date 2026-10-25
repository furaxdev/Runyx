# Usage (M0 + M1)

## Build

```bash
go build -o runyx ./cmd/runyx
```

## Hardware detection & recommendations

```bash
./runyx doctor       # CPU/RAM/GPU/VRAM report + best backend to use
./runyx recommend    # models from the default catalog that fit this machine
```

## Models

```bash
./runyx pull <owner/repo>[:file.gguf]   # download from Hugging Face
./runyx list                            # installed models
./runyx show <model>                    # size, source, tuning
./runyx tune <model>                    # (re)compute and cache tuning params
./runyx rm <model>                      # remove an installed model
```

`pull` also works with models already on disk from Ollama: point
`OLLAMA_MODELS` (or the default `~/.ollama/models`) and a future `runyx
import --ollama` scan will register them without re-downloading — the
scanning logic is in `internal/modelstore.ScanOllamaModels` today; wiring
it to a CLI subcommand is a short follow-up.

## Chat

```bash
./runyx run <model>
```

Pulls the model if it isn't installed, loads `llama-server` with
hardware-tuned parameters, and opens an interactive chat loop. Type
`/exit` to quit.

## Daemon + APIs

```bash
./runyx serve
```

Starts the HTTP server on `127.0.0.1:11535` (configurable in
`~/.config/runyx/config.toml` on Linux, the OS-equivalent elsewhere):

- **Ollama-compatible**: `GET /api/tags`, `POST /api/show`, `DELETE
  /api/delete`, `POST /api/generate`, `POST /api/chat`.
- **OpenAI-compatible**: `GET /v1/models`, `POST /v1/chat/completions`,
  `POST /v1/completions`, `POST /v1/embeddings`.

Example with `curl`:

```bash
curl http://127.0.0.1:11535/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"<model-name-from-runyx-list>","messages":[{"role":"user","content":"Bonjour !"}]}'
```

Requests from `127.0.0.1`/`::1` are allowed without a token by default.
Reaching the API from another machine (an explicit opt-in — change
`listen_addr` in the config) requires a device token; issuing/revoking
tokens via the CLI is tracked as a short follow-up (the `daemon.TokenStore`
API already supports it).

## Vision bridge (item 31) — optional OCR dependency

Image attachments to non-vision models are analyzed by an optional
small auxiliary vision model (pulled like any other model) plus local
OCR via [Tesseract](https://github.com/tesseract-ocr/tesseract), which
Runyx does not bundle or auto-install (unlike the llama.cpp/whisper.cpp
binaries, Tesseract has no single official per-OS/arch release
artifact). Install it yourself if you want OCR:

- Debian/Ubuntu: `sudo apt install tesseract-ocr tesseract-ocr-fra`
- macOS: `brew install tesseract tesseract-lang`
- Windows: the [UB-Mannheim installer](https://github.com/UB-Mannheim/tesseract/wiki)

Without Tesseract and without an auxiliary vision model installed,
Runyx reports a clear error rather than silently skipping image
analysis.

## Agent & tools (M4)

Enable the safe shell agent and/or the sandboxed code interpreter by
setting their work directories in `config.toml` (both are off by
default — Runyx never runs commands out of the box):

```toml
agent_work_dir = "/home/you/runyx-agent-workspace"
interpreter_work_dir = "/home/you/runyx-interpreter-workspace"
```

- `POST /api/runyx/agent/execute` `{command, args, reason}` — runs a
  shell command inside `agent_work_dir`. **Currently always refused**
  when reached through `runyx serve`: no interactive approval UI exists
  yet (see docs/BLOCKED.md). The policy/snapshot/audit/undo machinery
  itself is implemented and tested; only the "ask a human and wait" loop
  is missing.
- `POST /api/runyx/agent/undo` `{snapshot_id}` — resets the work dir to
  a prior git snapshot.
- `POST /api/runyx/interpreter/run` `{code}` — runs Python inside
  `interpreter_work_dir` (see the package doc comment in
  `internal/interpreter` for what this restricted execution does and
  does not isolate against).
- `GET /api/runyx/permissions` — what's currently enabled (shell agent,
  vision, voice, web search) and the list of device tokens.
- `runyx mcp-serve` exposes Runyx's own tools (currently just
  `web_search`) to any MCP-aware client over stdio.

## Voice, pairing & remote fallback (M5)

- `POST /api/runyx/voice/transcribe` (multipart `file`) and `POST
  /api/runyx/voice/synthesize` `{text}` — need `whisper-cli`/`piper` on
  PATH respectively; otherwise return a clear 503.
- `POST /api/runyx/pairing/qrcode` `{device_name}` returns a PNG QR
  code encoding a fresh device token + this machine's LAN address, for
  the Android app's pairing screen.
- `GET /api/runyx/remote/providers` / `POST /api/runyx/remote/chat`
  `{provider, ...rest is a normal chat completion body}` — opt-in
  remote-API fallback (item 28). No providers are configured by
  default; wiring one up currently means constructing a
  `remote.Router` with a real `Provider{BaseURL, APIKey}` (a config.toml
  section for this is a natural follow-up, not yet built).
- Android app: see `android/README.md`-equivalent instructions below —
  build it with Android Studio or `cd android && ./gradlew
  assembleDebug` (needs the Android SDK; this repo's CI installs it
  automatically).

## Bench, plugins, automation (M6)

```bash
runyx bench <model> [prompt]   # tokens/sec and time-to-first-token
runyx update                   # checks GitHub for a newer release
```

- `POST /api/runyx/arena` `{models: [...], prompt}` runs the same
  prompt against 2+ loaded models concurrently for a blind A/B/...
  comparison.
- Plugins are MCP servers Runyx launches itself — construct a
  `plugin.Manager` and `Start` a `plugin.Spec{Name, Command, Args}`
  (any program speaking MCP over stdio, including `runyx mcp-serve`
  itself); `GET /api/runyx/plugins` lists what's running, `POST
  /api/runyx/plugins/{name}/call` invokes one of its tools. No
  plugins run by default.
- Automation (`internal/automation`) has a cron-style `Scheduler`, a
  `FolderWatcher`, and an HMAC-verified `WebhookRegistry` — none are
  wired into `runyx serve`'s default config yet; use them by
  constructing them directly in a custom build, or wait for the
  config.toml + CLI surface that's a natural follow-up.
- `RUNYX_PORTABLE=1 runyx serve` keeps all data next to the binary
  (portable/USB-stick mode).
- See `scripts/package/README.md` for packaging (Docker, Homebrew,
  AUR, deb/rpm, winget) — manifests exist but weren't built/published
  from this sandbox.

## Known limitations in this milestone

- `runyx pull` always re-downloads rather than consulting mirrors; mirror
  fallback is a follow-up.
- The llama.cpp backend binary is fetched from GitHub releases on first
  use; if that fetch fails (no network, GitHub unreachable), `runyx run`
  / `runyx serve` report a clear error rather than silently retrying.
- `/api/pull` (server-side pull over HTTP) only confirms models already
  installed via the CLI; true server-side pulling is deferred to M2 when
  the web UI needs it.
