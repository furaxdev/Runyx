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
