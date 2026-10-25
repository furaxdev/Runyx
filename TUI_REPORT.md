# Runyx TUI -- status report

A from-scratch terminal UI for Runyx (TypeScript, no Ink/React/blessed),
talking to the real Go daemon over HTTP+SSE. This report is updated as
milestones land; it is not a promise of what will eventually exist --
only what is true right now.

## What works

- **Rendering engine (T0)**: double-buffered screen, cell-diff repaint,
  ANSI/SGR output, color-level and terminal-theme (light/dark)
  detection, full input parsing (keys, mouse, bracketed paste, focus),
  a dirty-flag render loop (coalesced to ~30fps while streaming).
- **Layout + widgets (T1)**: single-axis flex layout, virtualized
  scrolling, box/button/list/progressbar/spinner/text/textinput
  widgets, a mouse-hit-testing zone registry.
- **Markdown + selection + clipboard (T2)**: a markdown renderer
  producing styled, wrapped `RenderedLine[]` with source-span tracking;
  content-anchored selection (word/line/select-all, soft-wrap-aware
  copy, "copy as markdown", per-code-block copy) that survives
  scrolling and streaming; OSC 52 clipboard (tmux-DCS-wrapped) with a
  CLI-tool fallback.
- **Daemon client (T3 part 1)**: `src/api/config.ts` resolves
  `listen_addr` from the same `config.toml` the Go daemon reads;
  `src/api/daemon.ts` probes `/healthz` and spawns `runyx serve`
  (detached) if nothing answers, polling until healthy or a timeout;
  `src/api/client.ts` is a typed client for
  `/api/tags`, `/api/runyx/{conversations,dashboard,permissions,registry}`,
  and `/v1/chat/completions` (streaming), with field casing matching
  the Go structs exactly.
- **Chat screen (T3 part 1)**: a real, working conversation against
  the real daemon -- history persisted via
  `/api/runyx/conversations`, token-by-token streaming, Ctrl+X to
  abort. Deliberately mirrors `web/src/lib/Chat.svelte`'s fix for the
  "streaming cursor never detaches" demo-CI bug (see
  `docs/DECISIONS.md`): no `max_tokens` cap on a real conversation,
  and a 20s stall watchdog that shows "model is slow or stalled"
  instead of a spinner that can never resolve.
- **Dashboard / Models / Permissions screens + Ctrl+K palette (T3 part
  2)**: read-only views of `/api/runyx/dashboard` (loaded models,
  RAM/VRAM, backend), `/api/runyx/registry` (install state + hardware
  fit per model), and `/api/runyx/permissions` (shell-agent scope,
  vision/voice/web-search flags, paired devices -- device revocation
  is wired, Enter on a device sends the `DELETE`). `src/app.ts` is the
  app shell: Chat is the default screen; leaving any screen (Esc/q)
  opens the Ctrl+K-style palette to jump to another; Ctrl+C quits.
- **Command approval (T3 part 3)**: the shell agent previously
  refused every command outright (`agent.New(policy, nil)` -- see
  `docs/DECISIONS.md`'s "Generic async approval queue" entry). A new
  `internal/approval.Queue` bridges a blocked command execution to a
  human decision from any attached client; `src/screens/approvals.ts`
  polls `GET /api/runyx/approvals` every second and lets the user
  approve (`a`) or deny (`d`), with the full command detail shown.
- **Model pull progress (T3 part 3)**: the Models screen now streams
  `/api/runyx/models/pull`'s NDJSON progress into a real progress bar
  (`p` to pull the selected model, Ctrl+X to cancel).
- **Pairing screen (T3 part 3)**: issues a one-time device token via
  the new `POST /api/runyx/pairing/qrcode-matrix` (same token/host
  logic as the web UI's PNG route, refactored to share it) and renders
  the QR code's raw module grid as unicode half-blocks -- no QR
  encoding was reimplemented in TypeScript; `internal/pairing`'s
  existing, tested Go encoder is still the only one.

## What is tested

- 163 unit tests (`bun test`, see `tui/test/*.test.ts`): terminal
  buffer/diff, input parsing, layout/scroll math, widgets, markdown
  rendering, selection/clipboard (T0-T2), the client's SSE termination
  handling, `config.toml` parsing, approval/pull/pairing client calls,
  and the QR half-block rendering logic (T3).
- Go side: `internal/approval` (9 tests, including a genuine
  self-deadlock caught by `go test -race -timeout`), 3 new HTTP
  integration tests for the approval endpoints (end-to-end blocking
  approve, hash-mismatch rejection, disabled-by-default), and 3 new
  tests for the QR matrix endpoint/encoder. Full Go suite green with
  `-race`.
- `tsc --noEmit` is clean across the whole `tui/` tree.
- The compiled binary was smoke-tested against a real `runyx serve`:
  daemon-autostart from cold (no daemon running, `runyx` found on
  `$PATH`) reaching a healthy `/healthz`, and the "no model installed"
  error path. Not exercised end-to-end with a real model loaded and a
  live terminal (no TTY available in this environment to drive the
  interactive Chat/Dashboard/Models/Permissions/Approvals/Pairing
  screens visually) -- this is the biggest real gap in verification
  and should be the first thing a human checks before trusting this
  report. In particular: the approval flow's blocking behavior is
  verified at the HTTP-integration level (Go) and the polling/approve
  UI is typechecked and unit-tested piece by piece, but the full loop
  (a real shell command blocks, the TUI's Approvals screen shows it,
  approving it from the TUI actually lets the command run) has not
  been driven end-to-end with a real terminal attached.

## What is not built yet

- **Command-approval UI, Remote Control scopes**: the approval queue
  above is single-PC only -- per-device scopes (view-only /
  can-approve-edits / can-approve-commands) and the "require approval
  on this PC for dangerous actions" override from the Runyx Code
  mission's §7 need Remote Control's device identity threaded through,
  which is C2 scope in `docs/RUNYX_CODE.md`.
- **Mouse + text selection inside the Chat transcript**: T2's
  selection/clipboard engine is built and unit-tested against
  synthetic `RenderedLine[]`, but the Chat screen does not yet wire
  mouse drag events to it (no click-to-select, no double/triple-click,
  no Copy button) -- it currently only scrolls with Up/Down.
- **T4, done**: `runyx tui` / bare `runyx` on a real terminal now exec
  the sibling `runyx-tui` binary (`cmd/runyx/cmd_tui.go`, 5 new Go
  tests); `--dump-frame <path>` renders the Ctrl+K palette to SVG
  (scoped down from every screen -- see `docs/DECISIONS.md` for why);
  `THIRD_PARTY.md` has a `tui/` section (no runtime dependencies).
- **T4, still not done**: real cross-platform release builds (beyond
  the `bun build --compile` targets already in `package.json`) and
  hands-on terminal-compatibility verification (the matrix that exists
  is protocol-support reasoning, not tested against real terminal
  emulators) -- both need real target machines/terminals this sandbox
  doesn't have. `--dump-frame` only covers the palette, not
  Chat/Dashboard/Models/Permissions/Approvals/Pairing, whose draw
  functions are entangled with their own async network loads.

## Captures

One: `tui --dump-frame <path>` renders the Ctrl+K palette overlay to
SVG and was smoke-tested (produces a real SVG with the expected box/
text content, not just a blank background -- an earlier attempt was
silently blank because `screenToSVG` reads the front buffer, which
only updates via `fullRepaint()`/`diff()`, not the back buffer `draw`
functions write to directly). Every other screen still needs its draw
logic decoupled from its async load before it can be dumped the same
way. The Go daemon's own demo (`demo.yml`, screenshots on the
`demo-screenshots` branch) covers the *web* UI only, not this TUI.
