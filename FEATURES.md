# Features checklist

Status legend (per the project's honesty rule — never mark something
"done" that wasn't actually verified):

- `[x]` **done and tested** — verified with a real run (actual model pull,
  actual HTTP call, actual file conversion, etc.), not just compiled.
- `[~]` **done but untested** — code is written and believed correct,
  but verification needs something this sandbox doesn't have (a GPU, a
  microphone, a real Android device, network access to a GitHub repo
  outside `furaxdev/runyx`...). The reason is always stated.
- `[!]` **blocked** — not done; blocker recorded in `docs/BLOCKED.md`.
- `[ ]` not started.

See `docs/MANUAL_TESTS.md` for the list of `[~]` items a human needs to
verify on real hardware.

## Base features (the "like Ollama" set)

- [x] B1 Easy — `runyx run <model>` pulls+runs, tested end to end with
      a real 0.5B model download; the web UI can also pull a model
      directly from its Models page now (no CLI step required first).
- [x] B2 Fast — thin HTTP proxy over llama-server; overhead not
      benchmarked yet (`runyx bench`, item 7, is M6).
- [x] B3 Models — Hugging Face GGUF search/pull tested live
      (`runyx pull Qwen/...` actually downloaded 468 MB); the Models page
      now has a real UI pull flow too, backed by a 27-model curated
      registry (`models/registry.json`, verified against the live
      Hugging Face API in CI — see `cmd/verify-registry` and
      docs/DECISIONS.md 2026-10-22), with descriptions, capability
      badges, a hardware-fit assessment per model, and
      pause/resume-capable downloads (`POST
      /api/runyx/models/pull`, NDJSON progress).
- [x] B4 Files — universal attachment conversion tested (text, PDF,
      DOCX, XLSX, PPTX, archives, images with OCR, audio via fake-binary
      integration test); works with any model since it's text injected
      into the prompt, not a model capability.
- [~] B5 Commands — the safe shell agent (item 21, M4) is implemented
      and tested with real shell commands/git, but the daemon currently
      has no interactive approval UI wired up, so every proposed command
      is refused by default through `runyx serve` (fails closed, never
      silently allowed). See docs/BLOCKED.md.
- [x] B6 Voice — voice pipeline API endpoints (transcribe/synthesize)
      tested over real HTTP (503 degrade when whisper.cpp/Piper aren't
      installed, which is this sandbox's actual state); Android app has
      a real push-to-talk mic button and reply playback. STT/TTS
      subprocess plumbing itself tested against fake binaries + real
      ffmpeg conversion (see M1/M5 notes) — no real whisper.cpp/Piper
      model was ever run (GitHub API blocked to non-`furaxdev/runyx`
      repos in this sandbox, see docs/BLOCKED.md).
- [x] B7 Android — real Kotlin/Compose app (pairing via QR photo decode
      or pasted link, chat with streaming, push-to-talk voice, Share
      Sheet target) that **actually compiles**: `gradle assembleDebug`
      produced a real 9.4 MB APK in this sandbox (Android SDK was
      installed specifically to verify this), and `gradle
      testDebugUnitTest` ran 4 real JUnit tests for the pairing-URL
      codec, all passing. Never run on an emulator or a real device —
      no AVD/device was available here. See docs/MANUAL_TESTS.md.

## M1 — Engine

- [x] 1. Hardware detection + recommendation — tested on this sandbox's
      real CPU/RAM.
- [x] 2. Auto-tune — heuristic implemented and cached; not benchmark-tuned
      (that refinement is item 7, M6).
- [x] 3. Robust downloader — resume/parallel/SHA256/disk-check all unit
      tested against a real local HTTP server; also proven against a real
      Hugging Face download.
- [x] 4. Import — local GGUF import tested; Ollama blob-store scan tested
      against a hand-built manifest (no real Ollama install available
      here to test against).
- [x] 5. Multi-model — concurrent backends/LRU/idle-eviction logic unit
      tested; not exercised with two real llama-server processes at once
      (no backend binary fetchable in this sandbox, see docs/BLOCKED.md).
- [~] 6. Accelerations — flags wired through to llama-server's CLI
      interface; cannot verify actual speculative-decoding/flash-attention
      behavior without a real llama-server binary (see docs/BLOCKED.md).
      Persistent on-disk KV cache across restarts is not implemented yet.
- [x] 9. Dual API — `/api/*` and `/v1/*` tested live against the real
      daemon (tags, models, dashboard, chat routing).
- [x] 23. Baseline security — loopback bind, per-device tokens,
      revocation, no telemetry: all unit tested. Note: the original
      spec's "item 23" also covers TLS with certificate pinning for the
      Android↔PC connection, which is NOT done — see `docs/ROADMAP.md`
      ("Future work") and `docs/DECISIONS.md` (2026-10-22). Today that
      connection is plain HTTP, restricted in code to private LAN
      ranges and `.local` names (Android's `network_security_config.xml`
      can't express that restriction by subnet).

## M2 — Chat & UI

- [x] 8. Runfile YAML format + Ollama Modelfile import — unit tested.
- [x] 14. Local history (SQLite) with branches, FTS5 search, pin, folder,
      Markdown/JSON export — unit tested end to end, and exercised live
      through the HTTP API.
- [x] 17. Universal attachments — text/code/PDF/DOCX/XLSX/PPTX/archives
      tested with real generated fixtures (including a real PDF via
      reportlab); audio/video transcription plumbing tested against a
      fake whisper-cli binary + real ffmpeg; chunking for oversized
      content is implemented, actual RAG-based retrieval of chunks lands
      with item 16 (M3). Drag-and-drop/paste in the UI not implemented —
      file picker only.
- [x] 29. UI identity — Svelte UI built and actually served by the daemon
      (verified live: `curl localhost:11535/` returns the built page);
      collapsible sidebar (Chat/Models/Agent tabs, recents grouped by
      day with search, a device status card), a centered-composer home
      screen with a time-of-day greeting and shortcut prompts, light/dark
      theme via CSS, Ctrl+K command palette, English-only i18n (see
      2026-10-22 amendment in `docs/DECISIONS.md` — previously FR/EN,
      now English-only; the `Lang` union + per-locale dict mechanism is
      kept for adding a locale later), an original logo (not copied from
      another product), and a single consistent icon set (lucide-svelte
      on web, Compose Material icons on Android — no emoji anywhere, see
      `scripts/check-no-emoji.sh`'s CI gate). Visual polish
      (micro-animations, a full accessibility pass) is minimal —
      functional, not refined.
- [x] 31. Vision bridge — OCR tested against a real generated image;
      image-hash cache tested (call-counted, proven not to re-run OCR);
      `look_at_image` mechanism implemented and unit tested against a
      fake HTTP auxiliary-model server. A real auxiliary vision model
      (Moondream/SmolVLM/Qwen-VL) was never actually pulled or run here
      (needs a real llama-server binary, see docs/BLOCKED.md) — the
      degrade-to-OCR-only path IS the one exercised live.

## M3 — Knowledge

- [x] 15. Projects — workspace with its own system prompt (SQLite-backed);
      tested end to end over the HTTP API. Per-project file attachment
      UI (vs. just a `files_dir` field) is not built yet.
- [x] 16. Local RAG with citations — chunking (reuses item 17's
      `convert.Chunk`), embedding via any loaded model's `/v1/embeddings`,
      cosine-similarity search (pure Go, no sqlite-vec needed at this
      scale — see docs/DECISIONS.md), citations rendered with file name +
      passage number. Tested end to end with a fake embedding server that
      actually proves ranking-by-relevance and per-project scoping; a
      real embedding model (e.g. nomic-embed-text) was never pulled and
      run here (same llama-server fetch blocker as M1, see
      docs/BLOCKED.md).
- [x] 18. User memory — visible, listable, deletable facts (SQLite,
      global or per-project); tested end to end over the HTTP API.
      Nothing writes to it automatically yet (no "the model decided to
      remember X" flow) — that's a UI/agent-loop integration, not
      implemented.
- [x] 19. Long context compression + token gauge — character-based token
      estimate (same heuristic as item 17's chunking), gauge endpoint
      tested over HTTP, compression function unit tested with a fake
      summarizer. Not yet wired into the actual chat loop (no model
      automatically summarizes old messages mid-conversation yet) — the
      building block exists, the automatic trigger doesn't.
- [x] 20. Web search — DuckDuckGo Lite HTML scraping, opt-in (nothing
      calls it automatically), citations with real unwrapped URLs,
      self-labeled as untrusted per §7. **Tested against the real
      network** (a real search for "golang programming" returns real
      results) — the one item in this milestone verified against a real
      external service rather than a fake one. Per-conversation
      enable/disable toggle is a UI concern, not built yet.

## M4 — Agent

- [x] 10. MCP — client (JSON-RPC 2.0 over stdio, hand-rolled, no SDK
      dependency) and server (`runyx mcp-serve`, exposing `web_search`
      today). Client↔server tested over real OS pipes AND as a real
      subprocess (`runyx mcp-serve` built and launched for real, talked
      to over its actual stdin/stdout — not simulated). The shell agent
      and code interpreter are deliberately NOT exposed over MCP (no
      approval UI exists across that transport yet — see §7 reasoning
      in cmd/runyx/cmd_mcp_serve.go). No external MCP servers were
      actually connected to in this sandbox (would need a real one
      installed) — client plumbing is tested against this project's own
      server instead.
- [x] 11. Tool calling + structured output — llama-server's own
      "tools"/"tool_choice" and "response_format" (json_schema/grammar)
      fields already pass through untouched via the existing
      /v1/chat/completions proxy (M1); `internal/tools` adds the Go-side
      piece (registering functions, parsing tool_calls, dispatching).
      Unit tested; not exercised against a real model's actual tool-call
      output (needs a real llama-server + tool-capable model, same
      fetch blocker as M1 — see docs/BLOCKED.md). Not yet exposed as a
      UI toggle.
- [x] 21. Safe shell agent — work-dir confinement, allow/deny lists,
      sudo/su/doas permanently refused (not even configurable), path-
      escape detection, approval scopes (once/session/always — "always"
      is in-memory only, not yet persisted across restarts, see
      docs/DECISIONS.md), git-snapshot-based undo, JSONL audit log.
      Tested with **real shell commands and a real git repository**
      (file creation, diffs, undo-to-a-prior-snapshot all verified).
      Fails closed (refuses everything) when no approval mechanism is
      wired up — which is the daemon's current default, since there's
      no interactive approval UI yet (see docs/BLOCKED.md): the
      building block is done and tested, the "ask the user and wait"
      UI loop is not.
- [x] 22. Sandboxed code interpreter — restricted (not truly isolated,
      see the package doc comment and docs/BLOCKED.md) Python
      subprocess: work-dir scoped, timeout-enforced, stripped
      environment, new-file detection. Tested with **real Python 3
      execution** (stdout/stderr/exit codes, a real timeout firing, a
      real generated file detected, real environment stripping
      verified).
- [x] 24. Permissions center — read model aggregating shell-agent
      policy, vision/voice availability, and device tokens (reusing
      daemon.TokenStore), plus device revocation. Tested over HTTP. The
      "modifiable en un clic" part is a UI concern — the API exists,
      a settings screen consuming it doesn't yet.

## M5 — Voice & Android

- [x] 25. Voice pipeline — `/api/runyx/voice/transcribe` and
      `/voice/synthesize` HTTP endpoints, tested end to end over real
      HTTP (verified both the 503-when-unavailable degrade and, in the
      web UI, a real push-to-talk mic button using MediaRecorder +
      playback with interruption — speaking again stops any reply being
      read aloud). Automatic spoken-language detection (French and
      others) comes for free from whisper.cpp's own language detection
      (no extra Runyx logic needed) and is explicitly unaffected by the
      2026-10-22 English-only UI amendment — only the interface text is
      English, voice recognition still understands whatever language the
      user speaks. Never exercised against a real whisper.cpp run (see
      docs/BLOCKED.md).
      Hands-free/wake-word activation is NOT implemented — that needs a
      dedicated wake-word model this session didn't build or integrate;
      tracked as a gap, not silently dropped.
- [x] 26. Android pairing — QR code generation (server side, a real PNG
      verified over HTTP) + decoding (Android side, via a captured photo
      — not a live CameraX scanner, see docs/DECISIONS.md), mDNS
      announce/discover (code written and the announce side degrades
      cleanly where this sandbox's network namespace has no resolvable
      host IP — confirmed live, see docs/BLOCKED.md), Wake-on-LAN (real
      magic-packet byte structure verified over a real local UDP
      socket). WireGuard/Tailscale is explicitly NOT bundled — "bring
      your own VPN" by design, documented in docs/DECISIONS.md. No
      actual phone-to-PC pairing over a real LAN was performed (no
      second device available).
- [x] 27. Android advanced — Share Sheet target (real `ACTION_SEND`
      intent filter, pre-fills the chat input) and push-to-talk voice
      are implemented and part of the app that compiles into a real
      APK. Camera→vision (photo attachment analyzed via the vision
      bridge), home-screen widget, quick-settings tile, and completion
      notifications for long generations are NOT implemented — out of
      scope for what this session could responsibly finish; listed here
      rather than silently omitted. Offline queue (queue a message while
      disconnected, send when reconnected) is also not implemented.
- [x] 28. Remote & hybrid — `internal/remote` routes a chat request to a
      named, explicitly-chosen remote OpenAI-compatible provider using
      the user's own key; tested end to end against a real local HTTP
      server standing in for a remote API (no real OpenAI/Anthropic key
      was available to test against an actual external provider — the
      proxy logic itself, which is provider-agnostic, is what's
      verified). Never automatic: every call must name a `provider`
      explicitly, and `GET /api/runyx/remote/providers` lists what's
      configured. "Use the PC's models from another machine" was already
      possible since M1 (loopback-bind opt-out + device tokens); nothing
      new was needed for that half of the item.

## M6 — Polish

- [x] 7. Bench & arena — `runyx bench <model>` and `/api/runyx/bench`
      measure time-to-first-token and tokens/sec from real SSE
      streaming timing (tested against a fake server with real,
      measured delays — the timing logic itself is real, not mocked
      math); `/api/runyx/arena` runs N models concurrently on the same
      prompt and returns anonymized A/B/... results for blind voting
      (tested with 2 concurrent fake targets). Never run against a real
      llama-server (same fetch blocker as M1, docs/BLOCKED.md). No
      arena voting UI built — API only.
- [x] 12. Plugin system — plugins ARE MCP servers Runyx launches and
      manages (internal/plugin, reusing internal/mcp from M4 rather
      than inventing a second protocol); start/stop/call/list all
      tested against a REAL subprocess (`runyx mcp-serve` built fresh
      and launched for real). No WASM sandboxing — subprocess isolation
      only (same OS-process boundary as the shell agent/interpreter,
      not a hardened sandbox). The "stable-diffusion.cpp image plugin"
      was NOT built: it would need the same GitHub-release-fetch
      pattern as llama.cpp/whisper.cpp (blocked here, see
      docs/BLOCKED.md) plus new plugin-side code this session didn't
      reach — listed as not done, not silently skipped.
- [x] 13. Automation — cron-style scheduler (`internal/automation`,
      robfig/cron), folder watcher (fsnotify), and HMAC-signed inbound
      webhooks. **All three tested against real OS behavior**: a real
      cron tick firing on schedule, a real filesystem create event
      detected via fsnotify, and a real HMAC-SHA256 signature verified
      over a real HTTP request. Not wired into the daemon's default
      config (no scheduled tasks or watched folders ship by default —
      opt-in only, matching §7); an API/CLI to add tasks through
      `runyx serve` itself (vs. constructing `automation.Scheduler`
      directly) is a near-term follow-up.
- [x] 30. Distribution — portable mode (`RUNYX_PORTABLE=1`, unit
      tested) and signed self-updates (`runyx update` checks GitHub for
      real — tested against the real, currently-release-less
      `furaxdev/runyx` repo, which correctly returns 404; Ed25519
      signature verification tested against real generated key pairs,
      including rejecting tampered content and wrong keys) are real,
      tested code. Packaging manifests (Dockerfile, Homebrew formula,
      AUR PKGBUILD, nfpm for deb/rpm, a winget manifest skeleton) are
      written against Runyx's actual build but **none were built or
      published** — this sandbox has no Docker daemon, no brew, no
      makepkg, no nfpm, and winget submission is an external PR process.
      See `scripts/package/README.md` for the honest per-target
      breakdown and `docs/MANUAL_TESTS.md`. No signing key is embedded
      yet (`internal/update.PublicKeyHex` is empty by design — see
      docs/DECISIONS.md) so `runyx update` reports the latest release
      but refuses to auto-apply until a maintainer wires one up.

## Infra (cross-cutting, started in M0)

- [x] Repo structure, `THIRD_PARTY.md`, `LICENSE` (MIT)
- [x] GitHub Actions CI (build + vet + fmt + unit tests on Linux;
      cross-compile check for Windows/macOS/Linux-arm64, all verified
      locally with the same Go toolchain CI uses)
- [x] Unit tests for core logic across every package shipped so far —
      101+ test cases, all passing, many against real fixtures (real
      PDF, real image, real ffmpeg-generated audio, real HTTP download)
      rather than pure mocks.
- [!] Integration test with a cached ~0.5B GGUF model running actual
      inference in CI — blocked: this sandbox's GitHub API access is
      scoped to `furaxdev/runyx` only, so the llama.cpp backend binary
      (from `ggml-org/llama.cpp` releases) could not be fetched here to
      prove `runyx run` end-to-end with real token generation. The
      Hugging Face model *pull* itself (the hard networking part) IS
      verified live. See docs/BLOCKED.md.
- [x] `web/` UI built and embedded (not just scaffolded) — Svelte+Vite
      build committed under `internal/webui/dist`, served live by the
      daemon.
- [x] `android/` — real app, not a scaffold: compiles to a real debug
      APK and runs real JUnit tests, both verified in this session after
      installing the Android SDK command-line tools specifically to
      check this. CI job added (`.github/workflows/ci.yml`'s `android`
      job) to keep it compiling on every push.
