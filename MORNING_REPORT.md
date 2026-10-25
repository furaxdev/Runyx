# Morning report

Written and updated through the night. Nothing here is merged into
`main` -- every commit below is on a branch, waiting for you to open
and review the PR yourself.

**Last updated**: 2026-10-24, later in the night shift (demo bug's
actual root cause found and fixed -- see below). This file is being
updated incrementally as work lands; if a section looks sparse, that
work hasn't happened yet, not that it failed silently.

## Kaggle credentials -- the key does not authenticate

You gave `KAGGLE_USERNAME=furaxdev` and a key starting `KGAT_...`.
I wrote it to `~/.kaggle/kaggle.json` (600 permissions, outside the
repo, never committed -- verified with `gitleaks` and `git status`).

**It does not work.** Tested two ways, neither printing the key value:
- The installed Kaggle CLI (2.2.5) now requires an interactive OAuth
  browser login (`kaggle auth login`) for every command -- there is no
  non-interactive key-based path in this version at all, so even a
  *valid* classic username+key pair wouldn't authenticate through the
  CLI as installed.
- Dropping to the raw REST API directly (`curl -u
  furaxdev:<key> https://www.kaggle.com/api/v1/kernels/list`,
  bypassing the CLI entirely) returned `401 Unauthenticated` --
  so the key itself doesn't validate against Kaggle's API, independent
  of the CLI's OAuth requirement.

**What this means**: all of item 5 (Runyx Core / Runyx Code fine-
tuning on Kaggle) is blocked on a working credential. I wrote
`docs/MODELS_PLAN.md` anyway (the plan doesn't need a live credential
to write), and did not attempt any training run -- there's nothing to
run it against. Nothing was pushed to Hugging Face or Kaggle.

**What you need to do**: generate a fresh Kaggle API token from
https://www.kaggle.com/settings (Account -> API -> Create New Token),
and separately decide how you want this handled given the CLI's new
OAuth requirement -- the options are (a) run `kaggle auth login`
yourself interactively once and hand me the resulting cached
credential file, or (b) I drive the Kaggle REST API directly (not the
CLI) once I have a key that actually authenticates, which sidesteps
the OAuth requirement entirely since the API itself may still accept
key auth on endpoints the CLI's new wrapper doesn't expose that way --
untested, since the current key doesn't authenticate at all to check.

## What's done tonight, by branch and commit

### `ccr-8047f90c-ur2jz8` (the pre-existing TUI/demo branch)

Everything here was actually finished *before* tonight's session
started (see the conversation's earlier turns) and is unchanged
tonight except as noted:
- Demo CI fix (streaming cursor bug), TUI T3 (command approval, pull
  progress, pairing) and T4 (Go launch wiring, `--dump-frame`), and
  the composer redesign were already committed and pushed.
- Flaky CI note: `build-test-lint` failed once tonight
  (`TestSearchRealDuckDuckGo`, `internal/websearch` -- a real-network
  test unrelated to anything in that commit, which only touched
  `internal/approval`/`agent`/`api`/`daemon` + TUI files). Re-ran the
  failed job via the GitHub API; confirmed locally too (`go test
  ./internal/websearch/...` passes repeatedly here). Recorded as a
  known flake, not fixed in code (it's a real DuckDuckGo network call
  by design, inherently flaky in CI) -- **you should decide** whether
  to make that specific test skip-on-network-error or leave it as is;
  I didn't change test behavior without being asked to.

### `feat/runyx-code` (new branch, branched from the above; nothing
merged into `main`)

- `c6527e3` -- `docs/RUNYX_CODE.md` (architecture, tool schemas,
  threat model, C0-C3 staging) + `assets/mascot/` (sprites.json,
  animations.json, source svg/gif) copied in from your zip.
- `bbd9e45` -- `internal/memory` (level-1 memory: USER.md/MEMORY.md/
  per-project MEMORY.md, § separator, 2200-char cap enforced,
  audit.jsonl, secret redaction before write) + `internal/secretscan`
  (regex-based redaction: GitHub/AWS/Slack/OpenAI-shaped tokens, PEM
  key blocks, generic credential assignments). 18 tests.
- `087eae5` -- `internal/sleepinhibit` (anti-sleep: Linux via a held
  `systemd-inhibit` subprocess -- smoke-tested against the real binary
  on this machine; macOS via `caffeinate`; Windows via
  `SetThreadExecutionState`, cross-compile-checked only, not run on
  real hardware; a `Manager` on top with refcounting, 4h max-hold,
  and battery-threshold pause logic). 17 tests. Also hardened
  `.gitignore` (secrets, model weights) and installed `gitleaks` with
  a `.gitleaks.toml` allowlist for this repo's own fake test tokens --
  run before every commit from here on.

- `f653f19` -- `internal/codeagent`'s `Sandbox`: every file tool's
  path-containment gate. Rejects absolute paths, `..` traversal, and
  symlink escape (a symlink planted *inside* the project pointing
  outside it, caught by resolving every existing ancestor directory,
  not just the final path), plus a deny-list (`.env`, `.ssh`, `.aws`,
  `.gnupg`, `.config`, `*.pem`, `*.key`, `id_rsa`/`id_ed25519`,
  `credentials.json`, `kaggle.json`) enforced even inside the project
  root. 9 tests, including a real `os.Symlink`-based escape attempt.
- `26096cf` -- the C0 tool set and agent loop:
  `read_file`/`list_dir`/`search` (read-only), `write_file`/
  `edit_file`/`delete`/`run_command` (each gated by
  `internal/approval.Queue`, fails closed with no queue configured),
  a unified-diff applier for `edit_file` that refuses a hunk whose
  context no longer matches the file's current content (stale-diff
  protection -- a model can't blind-apply a diff against content it
  hasn't actually re-read), and `run_command` as a thin adapter onto
  the existing `internal/agent.Agent` rather than a second
  command-execution path. `Loop` ties it all together: plan -> tool
  call -> observation, a configurable step limit (default 20), and
  Stop via context cancellation; mascot-state hooks
  (thinking/working/done/error) with no rendering logic here. **Tested
  entirely against a scripted fake model, no network** -- one tool
  call then stop, a tool error surfaced without aborting the batch,
  the step limit, cancellation before the first model call, and the
  full state-transition sequence for one turn. 34 tests total in this
  package.

All five `feat/runyx-code` commits' CI is green (`ci.yml`, full Go
suite + race detector; this branch doesn't touch `tui/` or `web/` yet
so only the Go job runs).

**Not started yet, in the order the brief gives them**: GitHub tools,
the sessions database + `session_search`, screenshot/read_image,
Remote Control device scopes, skills, the mascot renderers (web/TUI/
Android), the "New session" screen, and the model fine-tuning (blocked
on the credential above). This is a multi-week mission; what's landed
so far is C0's foundational pieces (memory, secrets, anti-sleep,
sandbox, file/command tools, the agent loop with a fake-model
integration test) -- not yet enough to drive a real end-to-end turn
from any client UI, since no client is wired to this package yet.

## Demo / inference readability (item 2) -- root cause found and fixed, provable this time

**You were right to ask this be checked.** I first looked at the actual
text in the latest demo capture
(`docs/screenshots/light/03-conversation-response.png` on the
`demo-screenshots` branch, commit `9a308d9`) and it was still garbled
multilingual gibberish -- the same symptom as the original AMD-EPYC
bug, not fixed by the CPU-kernel-pruning workaround added earlier. I
had previously (wrongly) treated the demo as fixed once the CI
pipeline went green and the screenshot *timing* issue (cursor never
detaching) was resolved -- I never actually read the reply text in
that fix's own commit. That was a mistake, and the rest of this
section is the actual correction, not a second unproven claim.

**What I did**: wrote `.github/workflows/demo-diagnose.yml`, a
dedicated isolation matrix on GitHub's real runners (this sandbox has
no outbound access to `github.com/ggml-org`, so the AMD-EPYC-specific
behavior can't be reproduced here) testing CPU-kernel pruning on/off,
the chat template (`--jinja` on/off), and a control model
(Llama-3.2-1B-Instruct) against the Qwen demo model, each hitting its
own `llama-server` instance directly (bypassing Runyx's daemon) with
`"What is 2+2? Answer with one word."` at temperature 0.

**Result** ([run 40080651797](https://github.com/furaxdev/Runyx/actions/runs/40080651797)):
the CPU-kernel theory was wrong. Every Qwen2.5-0.5B-Instruct-GGUF
Q4_K_M variant -- pruned, unpruned, and with `--jinja` -- produced the
exact same garbled multilingual text. Both Llama-3.2-1B-Instruct-GGUF
variants answered "4" correctly, instantly, on the *same* runner in
the *same* job. Same CPU, same llama-server build, same kernel-pruning
state: one model correct, the other garbage. The demo's model choice
was the actual cause, not CPU-kernel selection. (The honest correction
is also recorded in `docs/DECISIONS.md`, appended to the original
wrong theory rather than replacing it silently.)

**What I fixed, on `ccr-8047f90c-ur2jz8` (commit `8409f5f`)**:
- `demo.yml`'s `MODEL_SPEC` switched to
  `bartowski/Llama-3.2-1B-Instruct-GGUF` -- the model actually proven
  to produce readable output on this CI's runners.
- Added `runyx run <model> --selftest`: asks the same answer-checkable
  question once (temperature 0, 16 max tokens) and exits non-zero if
  the reply doesn't contain "4"/"four", instead of dropping into the
  interactive chat loop. Unit-tested (`cmd/runyx/cmd_run_test.go`)
  against both real expected answers and the literal gibberish strings
  this bug produced, so the check itself is proven to catch the
  failure it's meant to catch.
- `demo.yml` now runs `--selftest` right after pulling the model, so a
  future broken model/build pairing fails the pipeline loudly instead
  of only showing up as unreadable prose in a screenshot a human has
  to notice.
- Flagged (not removed) `Qwen/Qwen2.5-0.5B-Instruct-GGUF` in
  `internal/hwdetect/recommend.go`'s catalog, since only the default
  `Q4_K_M` pull was tested, not every quantization of that repo.
  README and `docs/MANUAL_TESTS.md` examples updated to the
  proven-working model.

**Still not re-captured**: the demo screenshots on the
`demo-screenshots` branch are still the old, broken-model ones as of
this writing -- `demo.yml` needs to actually run again (push-triggered,
or `workflow_dispatch`) against the new model before they refresh. The
next push to `ccr-8047f90c-ur2jz8` that reaches `demo.yml`'s paths will
do that automatically; I'm not forcing an extra run just to regenerate
screenshots sooner.

## TUI T5/T6 (item 1)

The original TUI mission (earlier in this session) only ever defined
T0 through T4, and T4 is finished (see `TUI_REPORT.md`). There's no
T5/T6 scope on record to "continue" into. Decision (logged in
`docs/DECISIONS.md`): rather than invent arbitrary new milestones,
remaining TUI gaps already tracked honestly in `TUI_REPORT.md` (mouse
selection in Chat, real cross-platform builds, hands-on terminal
testing) stay tracked there, and TUI work for the Code section itself
happens as part of Runyx Code's own C0-C3 staging instead.

## What you should test yourself once a PR exists

(Expanded as more lands tonight.)

- A real Kaggle token that actually authenticates (see above) before
  any fine-tuning work can start at all.
- The demo screenshots on `demo-screenshots` haven't refreshed since
  the model switch -- push something that touches `demo.yml`'s paths
  (or trigger it manually) and check the new screenshots show a real,
  readable "4" or similar, not just that the pipeline went green.
- `internal/codeagent`'s loop has only ever talked to a scripted fake
  model -- nothing yet confirms a real local model (Qwen2.5-Coder or
  otherwise) actually emits well-formed `tool_calls` through
  llama-server's OpenAI-compat endpoint for these exact tool schemas.
  That's `docs/CODE_BENCH.md`'s job once there's a client to drive it.
- Everything already listed in `TUI_REPORT.md` and `docs/BLOCKED.md`
  from earlier in this session (real model inference, real GitHub
  token, pairing from a real phone, real OS sleep/wake, Windows/macOS
  binaries on real hardware).

## PR checklist

Branches, not merged, waiting for you:
- `ccr-8047f90c-ur2jz8` -- PR #1 already open (TUI + demo + composer).
- `feat/runyx-code` -- no PR opened; push and open one yourself when
  ready, per your instruction not to merge or open PRs for you.
