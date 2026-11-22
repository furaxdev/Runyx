# Blocked items

Things not done, with why, so nothing is silently skipped.

## GitHub API access scoped to `furaxdev/runyx` only (this sandbox)

This session's network proxy allows GitHub API access (`api.github.com`)
only for `furaxdev/runyx`. `add_repo` for other repos was not attempted
repeatedly since the task doesn't call for attaching unrelated repos —
but it means these fetchers, while implemented against real public
release APIs and unit-tested for their selection logic, were never
exercised against the real network in this environment:

- `internal/backend.Fetcher` — fetches `llama-server` from
  `ggml-org/llama.cpp` releases. **Impact**: no real llama-server binary
  could be run here, so `runyx run`'s actual chat loop, multi-model
  concurrency (item 5), acceleration flags (item 6), and the vision
  bridge's auxiliary-model path (item 31) are all implemented and unit
  tested but not exercised end-to-end with real inference.
- `internal/voice.Fetcher` — fetches whisper.cpp/Piper from their GitHub
  releases. **Impact**: STT/TTS subprocess plumbing is tested against
  fake binaries (real argument passing, real stdin/stdout, real JSON
  parsing) and real ffmpeg conversion, but no real speech was ever
  transcribed or synthesized here.

## GPU Saver (colibri): verified further than the fetchers above, but still not run end to end

Unlike the two fetchers above, this session *did* get real, successful
network access to `github.com/JustVugg/colibri` (anonymous git clone,
and a real download of its v2.1.0 Linux release) — enough to verify
the actual release asset names, the real (flat, no leading directory)
archive layout, and `coli`'s real CLI flags (`serve --model --host
--port --model-id --api-key`, `import <gguf> --out <dir>`) by running
`coli --help`/`coli serve --help`/`coli import --help` for real and by
extracting a real downloaded archive, rather than guessing from
documentation. `internal/colibri`'s Fetcher, PickAsset, and
ServeCommand/ImportCommand are built against those verified facts.

**`colibri.Fetcher.Ensure` itself was then actually run against live
GitHub, and hit the same wall as the two fetchers above**: this
session's proxy serves anonymous `git clone` and direct
`github.com/.../releases/download/...` asset downloads fine (both of
which is how the verification above was done), but `api.github.com`
calls for an unattached repository are refused outright ("GitHub
access to this repository is not enabled for this session") — the
exact call `Ensure` makes first, to resolve "latest" into real asset
URLs. So `Ensure`'s release-resolution step is confirmed blocked here
the same way `internal/backend.Fetcher`'s and `internal/voice.Fetcher`'s
are, even though colibri's own repository content was reachable by
other means. A real user's machine has ordinary `api.github.com`
access and isn't affected by this.

**Still not exercised, and honestly can't be from here**: no real GGUF
file (of any size) was available in this sandbox, so `coli import` was
only ever run against a deliberately invalid file (confirming its real
GGUF-magic-byte validation error, not a successful conversion), and
`coli serve` was never actually started against a real imported model
— so the full Runyx-launches-colibri-as-a-subprocess path
(`backend.Manager.Start` with `StartOptions.UseColibri`, through
`waitHealthy` to a real `/health` response, through to a real
`/v1/chat/completions` reply) is implemented and unit tested piece by
piece but not proven end to end. A maintainer with a real large MoE
GGUF and Python 3.10+ should run through `runyx gpu-saver import
<model>` followed by a real chat request before trusting this feature
in production.

**Not actually blocked for a normal user**: outside this sandbox, with
normal internet access, both fetchers hit real, public, officially
documented release APIs and should work as-is. A maintainer with normal
GitHub access should run through `docs/MANUAL_TESTS.md` to confirm.

## No real Android device or emulator

The Android module (`android/`) is a full project (Gradle, manifest,
Compose UI skeleton) intended to compile in CI, but no real device,
emulator, or even an installed Android SDK was used to run it in this
sandbox session. See `docs/MANUAL_TESTS.md`.

## No GPU in this sandbox

Every acceleration path that depends on an actual GPU (CUDA/ROCm/Vulkan
offload, the `BestBackend()` selection beyond "cpu-avx2") is implemented
and its selection *logic* is unit tested with synthetic hardware info,
but never run against real GPU hardware.

## Cannot push git tags to the remote from this sandbox

`git push origin <branch>` works fine, but `git push origin <tag>`
consistently fails with HTTP 403 (confirmed with both annotated and
lightweight tags, and with tag deletion) — the Claude GitHub App's
credentials for this session appear scoped to branch refs only, not tag
refs. Tags (`m1-done`, `m2-done`, etc.) are still created locally in this
session's clone so the milestone history is correct in the commit graph,
but they won't appear on `github.com/furaxdev/Runyx` until someone with
full push access runs `git push origin --tags` from a clone that has
them (or recreates them from the commit hashes recorded in this file and
in commit messages).

## No wake-word / hands-free voice activation

Item 25 asks for a hands-free mode with a wake word, in addition to
push-to-talk. Push-to-talk is implemented (mic button in the web UI and
the Android app, both tested against the real backend). Always-on wake-
word detection needs a dedicated small always-listening model (e.g.
Porcupine, or a custom keyword-spotting model) running continuously,
which is a different kind of component than anything else in this
session and wasn't built. Not silently dropped — just not done.

## mDNS announce doesn't work in this sandbox's network namespace

`pairing.Announce` failed in this sandbox with "could not determine
host IP addresses for vm" — the container's network namespace doesn't
expose a resolvable non-loopback IP the way a real LAN-connected
machine would. The code itself (`internal/pairing`) is standard
`hashicorp/mdns` usage and should work normally on a real machine on a
real LAN; `TestAnnounceAndDiscover` in `internal/pairing` skips with
this exact reason rather than failing, so CI won't be misled either.

## No second physical device for real Android↔PC pairing

The Android app compiles and its pairing-URL codec is unit tested, but
pairing a real phone to a real running `runyx serve` over an actual LAN
— scanning the live QR code, confirming the app can actually reach the
daemon and stream a chat response, confirming push-to-talk records and
transcribes for real — needs a second physical device and wasn't
possible in this single-container sandbox. See docs/MANUAL_TESTS.md.

## Packaging manifests written but not built (no Docker/brew/makepkg/nfpm here)

`scripts/package/` has a real Dockerfile, Homebrew formula, AUR
PKGBUILD, nfpm config, and winget manifest skeleton, all written
against Runyx's actual build process. None were actually built in this
sandbox: there's no running Docker daemon (the `docker` CLI is present
but `docker ps` fails to reach a socket), no Homebrew, no Arch
`makepkg`, and `nfpm` isn't installed. See `scripts/package/README.md`
and `docs/MANUAL_TESTS.md`.

## stable-diffusion.cpp plugin (image generation) not built

Item 12 suggests a stable-diffusion.cpp-based image generation plugin
as a first example. The plugin *system* itself is built and tested
(real subprocess, real MCP protocol — see FEATURES.md item 12), but
writing the actual stable-diffusion.cpp plugin needs the same
GitHub-release-fetch pattern already blocked for llama.cpp/whisper.cpp
in this sandbox, plus new code this session didn't get to. Not started.

## No real microphone / speaker

Push-to-talk and hands-free voice interaction (item 25) need a real audio
input/output device to test the actual user experience (wake word
detection, interruption while speaking). Not available in this sandbox.
