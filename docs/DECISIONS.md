# Decisions log

Deviations from the original brief, and why.

## 2026-10-21 — SQLite driver: pure-Go (`modernc.org/sqlite`) instead of cgo `mattn/go-sqlite3`

Keeps `CGO_ENABLED=0` cross-compilation trivial for Windows/macOS/Linux from
one Linux CI runner, at a small performance cost that doesn't matter for a
local, single-user daemon. Revisit only if we need SQLite extensions that
the pure-Go driver can't load.

## 2026-10-21 — `backend.Fetcher` (llama.cpp binary fetch) verified against
real URLs but not run end-to-end in this sandbox

Hugging Face downloads work from this sandbox and were tested live
(`runyx pull Qwen/Qwen2.5-0.5B-Instruct-GGUF:...` actually downloaded
468 MB and registered the model). `api.github.com`, which
`internal/backend.Fetcher` uses to find the latest llama.cpp release
asset, is blocked for this session's proxy (scoped to `furaxdev/runyx`
only) — unrelated to Runyx's design, just this sandbox's repo allowlist.
So `runyx run` could not be exercised against a real `llama-server` here;
it fails with a clear "no llama-server backend available" error instead,
which is the intended behavior when the fetch can't complete. The asset-
selection logic (`PickAsset`) is unit-tested with recorded GitHub asset
names, and the code is otherwise ready to run as-is on a normal machine
with full internet access. CI's integration test mocks the HTTP layer
instead of hitting the real network, so it stays deterministic.

## 2026-10-21 — Pure-Go cosine similarity instead of sqlite-vec for RAG (item 16)

The brief suggests `sqlite-vec`, but it's a loadable SQLite extension
(compiled C), which the pure-Go `modernc.org/sqlite` driver can't load
without cgo — the same reasoning that picked that driver in the first
place (see the entry above) applies here too. At the scale a local,
single-user RAG index actually reaches (a few thousand chunks per
project, not millions), a linear scan computing cosine similarity in Go
over all stored embedding vectors is well under a millisecond and avoids
the dependency entirely. Revisit only if a project's index grows large
enough (tens of thousands+ chunks) that linear scan becomes a visible
delay — an in-memory approximate index (e.g. HNSW) would be the next
step, not pulling in cgo.

## 2026-10-21 — Web search uses `lite.duckduckgo.com/lite/`, not
`html.duckduckgo.com/html/`

The brief mentions either SearXNG or DuckDuckGo. `html.duckduckgo.com`
was tried first and is unreachable from this sandbox's egress proxy
(connection resets / TLS handshake timeouts on every attempt);
`lite.duckduckgo.com/lite/` worked and needed only a GET with a `q`
query param (vs. the heavier `html/` endpoint's POST form), confirmed
against the real network. No API key, no tracking parameters added.
SearXNG wasn't wired up — it needs a running instance (public ones are
rate-limited and change availability over time; self-hosting is out of
scope for M3) — but `internal/websearch.Client` is small enough to swap
or extend with a second backend later if a user wants to point it at
their own SearXNG.

## 2026-10-21 — Shell agent's "always" approval scope is in-memory only
(not yet persisted)

`agent.Agent.alwaysApprovals` lives only for the process's lifetime.
Persisting it (to the SQLite history DB, keyed by reason + maybe
work-dir) is the obvious next step, but doing it safely needs a bit more
design than this session had room for: what happens when the *reason*
text is similar-but-not-identical across requests, whether approvals
should be scoped per-project, and how a user reviews/revokes persisted
"always" grants (the permissions center, item 24, is the natural home
for that UI once it exists). Tracked as a near-term follow-up, not
forgotten.

## 2026-10-21 — Android QR pairing decodes a captured photo, not a live
camera scan

A live scanning UI needs CameraX (a `PreviewView` + an `ImageAnalysis`
use case) wired up correctly, which is meaningfully more surface area
and more places to get wrong than a one-shot photo. For a first pairing
flow, the standard `ACTION_IMAGE_CAPTURE`-style photo picker
(`TakePicturePreview` in Compose) plus zxing's pure-Java decoder gets
the same outcome — the user photographs the PC's on-screen QR code —
with far less code and no extra heavyweight dependency. Revisit with
CameraX once the pairing flow needs to feel instant rather than
"take a photo, wait a second."

## 2026-10-21 — WireGuard/Tailscale is "bring your own VPN", not bundled

Item 28 mentions WireGuard/Tailscale for remote access. Both are
substantial pieces of software (WireGuard needs a kernel module or
userspace implementation and key management; Tailscale is a whole
separate daemon and account system) that don't belong vendored into
Runyx. The realistic integration is: Runyx binds normally on the LAN or
over whatever VPN interface the user already has running, and pairing
works the same either way since it's just an IP + port + token. No
code change was needed for this to already work; what's explicitly NOT
done is any Runyx-side WireGuard/Tailscale setup automation.

## 2026-10-21 — No real Ed25519 release key embedded in `internal/update`

`update.PublicKeyHex` is deliberately left empty rather than filled
with a placeholder/test key. A "signed updates" feature that silently
verifies against a fake key would look safe while verifying nothing —
worse than clearly refusing to apply updates until a maintainer
generates a real key pair, keeps the private half secret, and embeds
the public half at release-build time (e.g. via `-ldflags
"-X github.com/furaxdev/runyx/internal/update.PublicKeyHex=<hex>"`).
`runyx update` today checks for and reports a newer release, and tells
the user exactly why it won't apply it automatically.

## 2026-10-21 — `runyx` talks to `runyxd` over plain HTTP on 127.0.0.1, no
Unix socket

Simpler cross-platform story (Windows has no Unix sockets without extra
code), and the loopback-only bind plus a per-install token already satisfies
the "local only by default" security requirement. Revisit if we need to
support multiple OS users on one machine safely.

## 2026-10-22 — Android cleartext restricted to LAN/.local in code, not XML

Item 23 (TLS with certificate pinning) is still not implemented — Runyx
only ever talks plain `http://` between the Android app and a paired PC.
Blanket `android:usesCleartextTraffic="true"` was replaced with an
`android:networkSecurityConfig` pointing at
`android/app/src/main/res/xml/network_security_config.xml`, but Android's
`<domain>` entries only support exact hostnames/IPs and DNS-suffix
matching (`includeSubdomains`) — there is no CIDR/subnet syntax, so "allow
cleartext to 10.0.0.0/8 / 172.16.0.0/12 / 192.168.0.0/16" can't be
expressed declaratively for arbitrary LAN IPs the way it can for a named
domain. The actual restriction is enforced in code instead:
`Pairing.kt`'s `isPrivateOrLocalHost()` refuses to pair with (and
therefore the app never connects cleartext to) any host outside those
private IPv4 ranges, loopback, or a `.local` name — covered by unit
tests in `PairingTest.kt`. **TLS with certificate pinning (item 23)
remains tracked future work**, now also listed in `docs/ROADMAP.md`.

## 2026-10-22 — UI and docs in English only

The web UI, CLI (help/messages/errors/prompts/progress), Android app,
API error strings, logs, README and `docs/` are English-only. The
previous bilingual FR/EN `web/src/lib/i18n.ts` dict and in-app language
toggle were removed, but the **mechanism** is kept exactly as it was (a
`Lang` union type + one string table per locale) so adding a locale
later is a new union member and a new dict entry, not a rewrite. This
does **not** touch voice *recognition* (item 25): the speech-to-text
model still understands whatever language the user actually speaks —
only the interface text itself is English. `RUNYX_PROMPT.md` (the
original French spec) is kept as the historical record of what was
asked, amended in place at §1 and items 25/29 to note this change
rather than rewritten.

## 2026-10-22 — UI redesign: sidebar + home + model registry + icon set

Reworked the web UI's layout and navigation (collapsible sidebar with a
Chat/Models/Agent tab selector, recents grouped by day with search, a
device status card, a centered-composer home screen with a greeting and
shortcuts) and replaced the old "type a Hugging Face repo name" pull flow
with a real, curated model catalog.

- **Model registry** (`models/registry.json`, embedded via
  `internal/registry`): 27 hand-picked models spanning ~0.5B-72B,
  including vision and code-specialized models, each verified against the
  real Hugging Face API (repo exists, file exists, real byte size) —
  `cmd/verify-registry` does this and runs in CI
  (`.github/workflows/ci.yml`'s `verify-registry` job) so a renamed or
  removed upstream file fails CI instead of silently going stale. No
  model was invented; every entry's description reflects that model's
  actual public model card.
- **Download pause/resume**: `POST /api/runyx/models/pull` streams NDJSON
  progress over the request body. "Pause" is the client aborting the
  fetch — the request context cancels, `internal/downloader` stops
  mid-chunk, and the partial `.part` + resume-state sidecar survive on
  disk exactly as they do for CLI `runyx pull`. "Resume" is calling the
  same endpoint again; `Download()` already knows how to pick the saved
  chunk state back up. No new resume mechanism was built — this reuses
  the downloader's existing resumable-download support end to end.
- **Icon set**: lucide-svelte (ISC) on the web UI, Jetpack Compose's
  `material-icons-extended` (Apache-2.0) on Android — one stroke
  style/weight each, no emoji anywhere in the product (see the
  `scripts/check-no-emoji.sh` CI gate). Both are listed in
  `THIRD_PARTY.md`.
- **Logo**: an original simple mark (three offset bars suggesting
  motion), not derived from any other product's icon or logo.
- **"Agent" page**: split out of the old combined Settings page — shell
  agent status, vision/voice/web-search flags, and paired-device
  management (with a "Pair a device" entry point into the existing
  pairing flow) now live there; Settings is appearance-only (theme,
  reduce motion).
- **llama.cpp build pinning** (`internal/backend/fetch.go`,
  `RUNYX_LLAMA_CPP_TAG` env var): upstream llama.cpp has no separate
  "stable" release channel — every published GitHub release is a
  continuous-build nightly tag (`bNNNNN`), and the "latest" one is
  frequently a source-only milestone tag with no binaries at all
  (resolved via its `nightly-tag.txt` asset instead). A real demo.yml
  CI run hit a CPU-kernel correctness bug in build `b11669`: the exact
  same model and build gave a coherent reply on an Intel-CPU GitHub
  runner and endless multilingual gibberish — never emitting a stop
  token — on an AMD EPYC 7763 runner, because `ggml-cpu` dynamically
  dispatches to one of several per-microarchitecture compiled kernels
  at runtime based on the detected CPU. By default `runyx` still
  follows "latest" automatically (unchanged for normal users); setting
  `RUNYX_LLAMA_CPP_TAG` pins an exact tag instead, bypassing both
  "latest" resolution and the nightly-tag indirection. `demo.yml` pins
  it (currently `b11836`) for CI reproducibility; revisit this if/when
  upstream ships an actual stable channel.
  **Update**: pinning alone didn't fix it — a later run on build
  `b11836` hit the exact same gibberish on another AMD EPYC 7763
  runner, so it isn't specific to one bad build; something about the
  kernel ggml's loader picks for that CPU is wrong, and llama.cpp has
  no supported env var to force a different one (checked
  `ggml-backend-reg.cpp` and `ggml-cpu.cpp` directly — selection is
  `ggml_backend_load_best()` scoring each `libggml-cpu-*.so` by
  detected CPUID with no override hook). Worked around it instead:
  `internal/backend/fetch.go`'s `pruneToBaselineCPUKernel()`, gated by
  the `RUNYX_LLAMA_CPP_SAFE_CPU_KERNEL` env var, deletes every
  microarchitecture-specific `libggml-cpu-*.so` after extraction except
  the baseline (`libggml-cpu-x64.so` / `-arm64.so`), so the loader can't
  pick anything but the baseline — trading its speed for correctness.
  Off by default (normal `runyx` keeps full CPU-optimized performance);
  `demo.yml` opts in, trading speed for a 0.5B demo model where it
  doesn't matter.
  **Correction**: the CPU-kernel theory above was wrong. A dedicated
  isolation workflow (`.github/workflows/demo-diagnose.yml`) ran the
  same prompt (`"What is 2+2? Answer with one word."`, temperature 0)
  against `llama-server` directly, bypassing Runyx's daemon entirely,
  across a 5-way matrix: Qwen2.5-0.5B-Instruct-GGUF Q4_K_M with the
  CPU kernel pruned, unpruned, and unpruned-with-`--jinja`, plus
  Llama-3.2-1B-Instruct-GGUF Q4_K_M pruned and unpruned, all on the
  same real GitHub Actions AMD EPYC runner in the same job
  ([run 40080651797](https://github.com/furaxdev/Runyx/actions/runs/40080651797)).
  Every Qwen2.5-0.5B variant — pruned, unpruned, and with the Jinja
  chat template — produced the exact same garbled multilingual output.
  Both Llama-3.2-1B variants answered correctly ("4") in under a
  second. Since the *same* CPU, *same* llama-server build, and *same*
  kernel-pruning state produced a correct answer for one model and
  garbage for another, the kernel selection was never the actual
  cause — it's specific to this Qwen2.5-0.5B-Instruct-GGUF Q4_K_M file
  (quant artifact or a chat-template/tokenizer mismatch; not narrowed
  further). Switched `demo.yml`'s `MODEL_SPEC` to
  `bartowski/Llama-3.2-1B-Instruct-GGUF`, the model actually proven to
  work, and added `runyx run <model> --selftest` (asks this same
  answer-checkable question once, exits non-zero if the reply doesn't
  contain "4"/"four") plus a demo CI step that runs it, so a broken
  model/build pairing fails the pipeline loudly instead of only
  showing up as unreadable prose in a screenshot a human has to notice.
  `RUNYX_LLAMA_CPP_SAFE_CPU_KERNEL` is kept on in `demo.yml` anyway as
  a no-cost margin, but is no longer believed to be load-bearing for
  correctness. The small-model entry in `internal/hwdetect/recommend.go`
  still lists `Qwen/Qwen2.5-0.5B-Instruct-GGUF` since this hasn't been
  checked against every quantization of that repo, only the default
  pull's `Q4_K_M` — flagged there for whoever investigates next.
- **Chat CLI hang safety net** (`cmd/runyx/cmd_run.go`): the interactive
  `runyx run` chat loop had no HTTP client timeout and no cap on reply
  length, so a runaway generation (as above) hung the CLI forever with
  no way to recover short of killing the process — which is exactly
  what happened in CI (a 30-minute job timeout, not a clean failure).
  Added a 3-minute client timeout and a 1024-token cap on
  `max_tokens` as defense-in-depth: neither fixes a bad generation's
  *content*, but both turn an indefinite hang into a bounded, visible
  failure.
- **TUI scope and build decisions** (`tui/`): a from-scratch terminal UI
  for Runyx, in TypeScript, with a hand-built rendering engine (no Ink,
  React, or blessed). Given the size of this (a terminal rendering
  engine, markdown renderer, mouse-driven selection engine, and a
  parallel implementation of every web UI screen), several scope calls
  were made unilaterally and are recorded here rather than asked about,
  per the mission brief:
  - **Runtime**: Bun (`bun build --compile`) is the primary target —
    already present in this environment, ships a fast startup
    standalone-executable compiler, and needs no native addons for
    anything in this engine. A Node `--experimental-sea` fallback path
    is documented but not built/tested this pass (no second packaging
    pipeline was set up — see `tui/BUILD.md`).
  - **Zero runtime dependencies**: true as shipped. Grapheme
    segmentation uses `Intl.Segmenter` (built into both Bun and Node
    ≥16, no package). East-Asian/combining-width uses a small
    hand-written Unicode width table (`tui/src/term/width.ts`) derived
    from the East Asian Width property, not an npm package — cheaper to
    own than to audit a third-party one for this scope, and it's one
    file.
  - **API contract**: `api/openapi.yaml` documents the subset of
    `internal/api`'s HTTP surface the TUI actually calls (chat
    completions, registry, conversations/messages/projects, dashboard,
    permissions, pull progress) — not the full daemon API. TUI-side
    TypeScript types in `tui/src/api/types.ts` are hand-written to match
    it and the existing `web/src/lib/api.ts` field casing exactly
    (conversations/messages/projects are PascalCase, unmarshaled
    straight from Go struct fields with no json tags; registry/
    dashboard/permissions are snake_case). No codegen pipeline connects
    the YAML to the TS types — keeping them in sync is manual, flagged
    here as a known gap rather than silently assumed solved.
  - **Auth**: `internal/daemon/auth.go`'s `LocalBypass` already accepts
    any loopback request with no token, so the TUI does not implement
    token acquisition/storage for the common case; it still *sends* a
    locally-cached pairing token when one exists (for parity with a
    non-default `LocalBypass: false` deployment), but doesn't block
    startup on obtaining one.
  - **Daemon discovery/autostart**: no existing "is it running, what
    port" mechanism exists in the Go daemon (checked `cmd_serve.go` and
    `internal/config`) beyond the configured `listen_addr`. The TUI
    reads the same `config.toml` the daemon reads, probes
    `GET /healthz`, and if that fails, spawns `runyx serve` itself
    (detached) and polls until healthy or a timeout, rather than Runyx
    gaining a new lockfile/PID-discovery protocol in this pass.
  - **T3 part 1 (implemented)**: `tui/src/api/config.ts`,
    `daemon.ts`, and `client.ts` implement the discovery/autostart
    paragraph above and a typed HTTP+SSE client for the daemon's
    `/api/*`, `/api/runyx/*`, and `/v1/chat/completions` routes (field
    casing matches the Go structs exactly — PascalCase for
    conversations/messages via `encoding/json`'s default, snake_case
    where `internal/api/*.go` sets explicit json tags). `config.ts`
    parses `config.toml`'s `listen_addr` with a couple of regex lines
    rather than pulling in a TOML library: the file Runyx itself writes
    (`internal/config`) is always a flat key="value" table, never
    nested tables or arrays, so a parser is more generality than this
    needs. `tui/src/screens/chat.ts` is the first real screen: it
    deliberately mirrors `web/src/lib/Chat.svelte`'s streaming contract
    (no `max_tokens` cap by default, a 20s stall watchdog that shows
    "model is slow or stalled" instead of a spinner that never ends,
    Ctrl+X to abort) — see this file's "streaming cursor never
    detached" entry for why that pairing matters: both clients hit the
    exact same unbounded SSE proxy. `tui/src/main.ts` wires
    boot -> ensure daemon -> pick the first installed model -> run the
    Chat screen; Models/Dashboard/Permissions/Pairing/command-approval/
    the Ctrl+K palette are not built yet (tracked in `TUI_REPORT.md`).
  - **Go-side launch wiring**: bare `runyx` and `runyx tui` exec a
    sibling `runyx-tui` executable (built from `tui/`) when stdout is a
    TTY, found next to the `runyx` binary or on `$PATH`; `runyx run
    <model>` keeps its existing plain chat loop (now with the hang
    safety net) rather than also redirecting into the TUI, to avoid
    changing that command's documented/scripted behavior — the TUI's
    own Chat screen is reached via `runyx tui`, not by overloading
    `run`. Embedding the compiled TUI binary into the Go binary (so
    there's only one executable to ship) was considered and rejected
    for this pass: it would require cross-compiling and `go:embed`-ing
    three platform-specific TUI binaries from Go's own build, which
    `scripts/build-ui.sh`'s existing single-platform CI doesn't support
    yet — left as a follow-up, noted in `FEATURES.md`.
  - **Testing**: no real-PTY end-to-end test was added. A `node-pty`/
    similar dependency would contradict the zero-runtime-dependency
    goal (native bindings) and this environment has no way to validate
    behavior against the 10 real terminal emulators the brief lists —
    unit/property tests run against the buffer and parsers directly
    (feeding synthetic byte sequences in, asserting grid/state out)
    instead. The terminal compatibility matrix in `FEATURES.md` is
    protocol-support reasoning (what each terminal is documented to
    support), not hands-on verification, and is labeled as such.

## Demo CI: streaming cursor never detached (Playwright timeout)

After the AMD-EPYC CPU-kernel-correctness fix (`RUNYX_LLAMA_CPP_SAFE_CPU_KERNEL`,
pinned `RUNYX_LLAMA_CPP_TAG`), the demo job still failed, but with a
different symptom: Playwright timed out waiting for `.cursor` to detach
in `scripts/demo/screenshot.mjs`, instead of the earlier "endless
multilingual gibberish."

- **Diagnosis without Playwright**: `demo.yml` gained a "Probe streaming
  endpoints (SSE diagnostics)" step that hits `/v1/chat/completions` and
  `/api/chat` directly with `curl -N` right after the daemon reports
  healthy, logging time-to-first-token, event count, events/sec, and
  whether a stream-end signal (`data: [DONE]` or `"done":true`) ever
  arrives — separating "the model/server is slow but finishes" from "the
  frontend never notices the stream ended," independent of whatever the
  browser-side bug turns out to be.
- **Root cause found in the frontend, not just speed**: `web/src/lib/api.ts`'s
  `streamChat` sent no `max_tokens` at all (`cmd/runyx/cmd_run.go`'s own
  `chatMaxTokens` safety net only covers the CLI's `runyx run` loop, never
  the web UI), and `Chat.svelte` only flips `streaming = false` once that
  unbounded request's promise resolves — with nothing watching for a
  stalled/never-ending stream, a slow or runaway generation left the
  `.cursor` element mounted indefinitely, which is exactly what Playwright
  saw. Fixed by:
  - `api.ts`: `streamChat` now accepts optional `maxTokens`/`temperature`
    and only includes them in the request body when given — no change to
    real conversations' defaults.
  - `Chat.svelte`: wires an `AbortController` (so a Stop button can cancel
    a request stuck on a legitimately-slow model) and a stall watchdog —
    if no delta arrives for 20s, it shows "Model is slow or stalled" with
    the Stop button, instead of a cursor that spins forever with no
    explanation. The watchdog never aborts on its own; a real model can
    legitimately take a while.
  - `screenshot.mjs`: passes `demoMaxTokens`/`demoTemp` query params (read
    by `Chat.svelte`, ignored unless present) to make the demo's one
    scripted reply short and deterministic (default 64 tokens, temp 0)
    instead of depending on however long an uncapped generation takes on
    CI's deliberately-de-optimized "safe" CPU kernel; the Playwright
    cursor-detach timeout was also raised to 300s as a ceiling against a
    genuine hang, not a budget expected to be needed in full.
  - Added `web/src/lib/api.test.ts` (Vitest, new `web` dependency) with a
    fake SSE `ReadableStream` to assert `streamChat` resolves on both
    `[DONE]` and plain connection-close, and that an aborted signal
    rejects instead of hanging — the frontend-side regression test for
    this exact bug shape. Wired into `ci.yml` as a new `web` job
    (`npm test` + `npm run build`); there was no CI job exercising `web/`
    at all before this.
- **Known gap, not fixed here**: `internal/api/ollama.go`'s `handleChat`
  decodes `OllamaChatRequest.Options` but never forwards it upstream, so
  `/api/chat` callers' `num_predict`/`temperature` are silently ignored
  today. Out of scope for this fix (the web UI uses `/v1/chat/completions`,
  not `/api/chat`) but left here so it isn't rediscovered as a mystery
  later.
- **CI noise**: `demo.yml`'s push trigger gained `paths-ignore` for
  `tui/**`, `docs/**`, `**.md` — TUI/doc-only commits can't change the
  daemon or web UI the demo exercises, so they no longer spend a full
  Playwright run (the existing `concurrency.cancel-in-progress` already
  cancelled the superseded run, but not starting it is strictly better).

## Generic async approval queue (`internal/approval`), replacing the shell agent's `nil` ApprovalFunc

`docs/BLOCKED.md` had flagged this precisely: `internal/daemon.New`
wired the shell agent with `agent.New(policy, nil)`, and `nil` means
"fail closed" per `resolveApproval` -- every command was refused
outright, because there was no bridge between a command execution
blocking inside one HTTP request's goroutine and a human looking at a
UI deciding, in a second, later HTTP request.

- `internal/approval.Queue` is that bridge: `Submit(ctx, req)` blocks
  the caller until `Resolve(id, hash, decision)` is called for it, its
  own 10-minute expiry passes, or `ctx` is cancelled -- always
  resolving an unanswered wait as denied, never as an implicit allow.
  `Resolve` fails closed on an unknown id, an expired request, or a
  hash that doesn't match the request's `Hash` (sha256 of
  `kind+"\x00"+detail`, computed by `NewRequest`, never trusted from a
  caller) -- so a command can't be approved after its content changed
  from what was shown.
  - Found and fixed a genuine self-deadlock while writing this:
    `List()` locked `q.mu` then called `clock()`, which also locked
    `q.mu` -- `sync.Mutex` isn't reentrant, so this hung forever the
    moment a second goroutine (`Submit`) also wanted the lock. Fixed
    by splitting `clock()` (locks) from a `clockLocked()` the already-
    locked callers use instead. Caught by `go test -race -timeout`,
    which dumps every goroutine's stack on a hang -- exactly how it
    was found.
- `internal/agent.ApprovalFunc`'s signature gained a `context.Context`
  parameter (`func(ctx, Request) (bool, ApprovalScope)`, was `func(Request)
  (bool, ApprovalScope)`): without it, an approval implementation that
  blocks on something like `Queue.Submit` had no way to honor the
  original HTTP request's cancellation, and would keep waiting up to
  the full 10-minute expiry after the client was already gone. This
  is a breaking change to the type, contained entirely within
  `internal/agent` and its own tests (nothing outside the package
  constructed an `ApprovalFunc` yet, since the daemon's only call site
  passed `nil`).
- `internal/daemon.New` now always creates `d.Approvals =
  approval.NewQueue()` (even with the shell agent disabled, so the
  HTTP endpoints have something to report "no pending approvals"
  against) and wires the shell agent's `ApprovalFunc` to
  `approvalAdapter(d.Approvals)` instead of `nil`.
- New routes: `GET /api/runyx/approvals` (list pending) and `POST
  /api/runyx/approvals/{id}/decide` (resolve one, 409 on any failure
  mode -- unknown id, hash mismatch, expired). Wired into
  `internal/api.AgentExtras` alongside the existing agent/interpreter
  routes.
- TUI: `tui/src/screens/approvals.ts` polls the list every second and
  lets the user approve (`a`)/deny (`d`) the selected request, showing
  its full detail; added to the Ctrl+K palette.
- **Scope note**: this is the single-PC "ask whoever is looking at a
  Runyx UI right now" case from the Runyx Code mission's §9/§7. It
  does *not* yet implement per-device scopes (view-only / can-approve-
  edits / can-approve-commands) or the "require approval on this PC
  for dangerous actions" override from §7 -- those need Remote
  Control's device identity threaded through `Resolve`, which is C2
  scope in `docs/RUNYX_CODE.md`, not part of finishing this pending
  item.

## TUI pairing screen reuses the Go QR encoder's raw module grid, not a reimplementation

The web UI renders pairing as a PNG `<img>` from
`GET/POST /api/runyx/pairing/qrcode`. The TUI has no image decoder, and
hand-rolling QR encoding (Reed-Solomon error correction, mode/version
selection) in TypeScript would duplicate -- and risk subtly
mis-duplicating -- logic `internal/pairing` already gets from a real,
tested library (`github.com/skip2/go-qrcode`). Instead:
`pairing.Info.QRCodeModules()` calls the same library's `Bitmap()`
method to get the raw `[][]bool` module grid, and a new endpoint
(`POST /api/runyx/pairing/qrcode-matrix`) returns `{url, modules}` as
JSON -- same token issuance and loopback-host refusal as the PNG
route (both now share `buildPairingInfo`, so they can't drift out of
sync). The TUI's `drawQRCode` renders two module rows per terminal row
with unicode half-blocks (▀▄█), the same "2 pixels per character"
technique the mascot's sprite renderer uses (§1 of
`docs/RUNYX_CODE.md`) -- a QR code is just another bitmap to a
terminal. Not yet done: the Android app's own QR *scanning* path
(decoding a photo) is unrelated and already existed; this only adds a
second way to *display* the code.

## T4: Go-side launch wiring and --dump-frame, scoped down from the original plan

- `cmd/runyx/cmd_tui.go`: bare `runyx` with no arguments, on a real
  terminal (checked via `os.Stdout.Stat()`'s `ModeCharDevice` bit --
  no extra dependency for this), and the new explicit `runyx tui`
  subcommand both exec the sibling `runyx-tui` binary (found next to
  the running `runyx` executable, then on `$PATH`), inheriting stdio.
  `runyx run <model>` is untouched, per the decision already recorded
  above. A non-zero exit from the child is carried back as a typed
  `ExitCodeError` rather than `cmdTUI` calling `os.Exit` itself --
  doing so in-process would kill the test binary too, making that path
  untestable; `main()` is the only place that calls `os.Exit`.
- `--dump-frame <path>`: renders one static frame to SVG (via the
  existing `screenToSVG`, built in T0) and exits, for visual
  regression without a live terminal. Scoped down from "every screen"
  to just the Ctrl+K palette: every other screen's `draw` closure is
  entangled with its own network-fetching async load (Chat even blocks
  on `ensureConversation` before `setDrawFn` is ever called), and
  decoupling each one is real work left for when per-screen snapshot
  tests are actually needed (e.g. alongside the mascot's own per-state
  CI snapshots). Caught a real bug while wiring this up: `screenToSVG`
  reads the *front* buffer, but `drawPalette` (like every draw
  function) only writes the *back* buffer -- front only updates via
  `diff()`/`fullRepaint()`. The first attempt produced an SVG with
  nothing but the background rect; fixed by calling
  `screen.fullRepaint("truecolor")` before converting, which is now
  the documented pattern for any future non-interactive render.
- Not done, and not attempted here: actual cross-platform release
  builds (beyond the `bun build --compile` targets already in
  `tui/package.json`), and hands-on verification against real terminal
  emulators -- both need real target machines/terminals this sandbox
  doesn't have. `THIRD_PARTY.md` gained a `tui/` section (no runtime
  dependencies at all, by design).

## Composer redesign: one component, visually verified with a real (if faked-data) daemon

Requested mid-session, scoped to the existing web UI (`Composer.svelte`
is already shared by Home's centered composer and Chat's bottom dock;
"the Code section" doesn't exist yet -- it'll get this same component
for free once built, nothing extra needed there).

- Rounded rect (24px radius, no hard border -- 1px subtle `--border` +
  `--shadow`, `--bg-elevated` background, which is already "slightly
  lighter than the page" in both themes), 2-row layout (textarea on
  top, toolbar below), auto-growing from a 2-line rest height up to 8
  lines (`autoGrow()` sets `textarea.style.height` from `scrollHeight`,
  clamped, then scrolls past that) instead of the old single-line
  `nowrap`+ellipsis textarea.
- Toolbar: round `+` button (file attach, unchanged behavior --
  `Paperclip` dropped from `icons.ts`, now unused anywhere), the model
  picker restyled as a borderless `--bg-sunken` pill (was bordered
  `--bg-elevated`), a spacer, round mic button, round send button
  (grey/`--bg-sunken` at rest, `--accent` only once `hasContent` is
  true -- previously `disabled` only ever meant "streaming", with no
  visual tie to emptiness at all).
- Focus ring via `color-mix(in srgb, var(--accent) 35%, transparent)`
  on `:focus-within`, no new CSS variables needed for it.
- Mobile (`max-width: 640px`): 12px side margin, 44px round buttons,
  and the model pill gets `max-width: 140px` -- found and fixed a real
  overflow bug here while visually verifying: the pill's flex item had
  no `min-width: 0`, so on a 390px viewport it refused to shrink below
  its content width and pushed the mic/send buttons out past the
  composer's own rounded border. Wrapping it in a
  `.model-select-wrap { min-width: 0; overflow: hidden; }` div inside
  `Composer.svelte` fixed it.
- **Verified visually, not just by typechecking**: built the real Go
  binary with the rebuilt UI embedded, started a real daemon with a
  faked `models.json` entry (no real GGUF needed just to see the
  composer with a model selected -- real inference wasn't being
  tested), and drove it with Playwright at desktop (light + dark) and
  390px-mobile viewports, including typing multi-line text to confirm
  the grow/shrink and the send button's grey-to-accent transition
  actually happen. This is how the mobile overflow bug above was
  caught -- it wasn't visible from the code alone.
- Demo screenshots: not manually regenerated here -- `demo.yml` already
  refreshes `docs/screenshots` (the `demo-screenshots` branch) on every
  push that touches non-`tui`/non-`docs` paths, so pushing this
  `web/src` + `internal/webui/dist` change triggers that pipeline the
  normal way.
- **"Secure max" hardening pass**: an unscoped request to make Runyx
  as secure as reasonably possible, so this was a real audit (read the
  actual code against nine specific risk areas: HTTP binding/CORS,
  backend-binary download integrity, pairing tokens, shell-agent
  policy, attachment handling, the auto-updater, secrets in logs, SQL/
  FTS injection, file permissions) rather than a generic pass. Three
  were real, not theoretical:
  - **Critical: `daemon.TokenStore`'s `LocalBypass`
    (`internal/daemon/auth.go`)** trusted any request whose TCP
    connection was loopback, with no check on what the request was
    actually addressed to. A webpage open in the user's browser could
    `fetch("http://127.0.0.1:<port>/...")` directly (its `Origin`
    header would show the real external site, uncheck), or a page on
    an attacker-registered domain that resolves to 127.0.0.1 (DNS
    rebinding) could do the same while its `Host` header still names
    that domain -- either way the connection is loopback even though
    the daemon was never the page's own origin, and the full local API
    (shell-agent approval requests, model pull/delete, chat history
    read/write) was reachable with zero friction, no user interaction
    beyond having the tab open. Fixed by `isGenuineLocalRequest()`:
    the bypass now only applies when the `Host` header itself names a
    loopback host (catches DNS rebinding) *and* any `Origin` header
    present also names one (catches a direct cross-origin fetch).
    Never affects a genuinely remote client (a paired phone, LAN
    access) -- those were never loopback at the TCP layer and so never
    reached the bypass in the first place. `TokenStore.Valid` also
    switched from `==` to `subtle.ConstantTimeCompare` while in there
    (low practical risk given 256-bit tokens, but free to fix).
  - **High: the shell-agent deny-list (`internal/agent/policy.go`)**
    compared `exe == "sudo"` literally -- `/bin/sudo`, `./sudo`, or
    (Windows) `SUDO.EXE` all name the exact same program but none
    matched, so a command nonNegotiableDenyPrefixes exists specifically
    to refuse *before any approval prompt* instead reached the prompt
    like any other command. `matchesDenied()` now compares
    `filepath.Base`, lowercased, with Windows executable suffixes
    stripped.
  - **High: `backend.Fetcher.Ensure` (`internal/backend/fetch.go`)**
    executed the cached `llama-server` binary on nothing more than
    `os.Stat` succeeding -- no integrity check at all, on a binary this
    is the one thing in Runyx that runs as a trusted subprocess with
    full access to whatever the daemon can reach. llama.cpp's releases
    don't publish per-asset checksums or signatures to verify against
    (checked), so this isn't full supply-chain signing -- it's
    trust-on-first-use hash pinning (`verifyOrPinBinary`): the SHA-256
    of a freshly-extracted binary is recorded in a `.sha256` sidecar,
    and every later "already cached" check (including the
    releases-API-unreachable fallback path, which used to trust
    *anything* on disk under the right directory name) must match it
    or the binary is treated as not-cached and re-fetched. Also
    verifies a GitHub-reported asset `digest` field against the
    downloaded archive when one is present, which costs nothing and
    activates automatically if llama.cpp's releases ever start
    including one.
  - **Medium, found while auditing, not in the original nine areas:
    `internal/convert`'s `readZipMember`** (shared by the .docx/.xlsx/
    .pptx converters) read a zip member's decompressed contents with
    `io.ReadAll` and no size bound at all -- a classic zip-bomb DoS: a
    small uploaded attachment could decompress to gigabytes and exhaust
    memory. Bounded via `io.LimitReader` at 200MB (far beyond any real
    Office XML part, so no legitimate file is affected).
  - Also tightened, lower severity: the shell agent's `audit.jsonl` and
    the command `Result` it returns now run every field through
    `internal/secretscan.Redact` (a command's `Reason`/`Args`, or its
    own stdout/stderr, could contain a secret the model was never
    asked to protect); `audit.jsonl` and `internal/history`'s
    `runyx.db` (every conversation the user has ever had) are now
    written `0o600` instead of the sqlite driver's/previous code's
    default (typically world-readable).
  - Attachment upload path handling (`sanitizeBase` -> `filepath.Base`
    plus separator stripping) and the SQL/FTS query construction in
    `internal/history` were checked and are already sound -- no change
    needed, named here so "checked, found fine" isn't silently
    indistinguishable from "not checked."
  - All of the above has new, passing tests proving the specific
    exploit each one closes (not just that the happy path still
    works) -- see `internal/daemon/auth_test.go`,
    `internal/agent/policy_test.go` and the new cases in
    `internal/agent/agent_test.go`, `internal/backend/fetch_test.go`,
    and `internal/convert/convert_test.go`.
