# Manual tests

Things marked `[~]` ("done but untested") in `FEATURES.md` that a
human should verify on real hardware with normal internet access. Updated
at the end of every milestone.

## After M1/M2 (do these first)

1. **Real llama.cpp backend fetch + run**: on a normal machine (not this
   sandbox), run `runyx doctor` then `runyx run bartowski/Llama-3.2-1B-Instruct-GGUF`.
   Confirm it downloads a real `llama-server` build and you get an actual,
   readable chat response (`runyx run <model> --selftest` gives a quick
   pass/fail on this without reading prose yourself — see
   docs/DECISIONS.md's note on the Qwen2.5-0.5B demo model that looked
   fine but wasn't).
2. **Multi-model + idle eviction**: `runyx pull` two small models, load
   both via two concurrent chats (or two `/v1/chat/completions` calls),
   confirm both stay loaded, then wait past `idle_unload_seconds` (config)
   and confirm the idle one unloads (check `runyx list`/process list).
3. **GPU offload**: on a machine with an NVIDIA/AMD/Apple GPU, run
   `runyx doctor` and confirm it's detected, then `runyx run` a model
   that fits in VRAM and confirm it's noticeably faster / the GPU shows
   load (`nvidia-smi`, Activity Monitor, etc.).
4. **Whisper.cpp STT for real**: install/pull a real whisper model,
   attach a real audio file (a voice memo) to a chat in the web UI, and
   confirm the transcription is accurate.
5. **Piper TTS for real**: same, for text-to-speech once item 25's voice
   UI exists (currently only the subprocess wrapper is implemented).
6. **Vision bridge with a real auxiliary model**: `runyx pull` one of
   the models in `internal/vision.Recommended` (e.g. Moondream2), attach
   a photo in the web UI, confirm you get a real description instead of
   OCR-only degraded mode.
7. **OCR install path**: on a machine without Tesseract installed,
   confirm attaching an image gives the documented clear error, then
   install Tesseract per `docs/USAGE.md` and confirm OCR starts working.
8. **Web UI across themes/OS**: open the UI in a browser with the OS set
   to light and dark mode, confirm the theme switches automatically, and
   that the manual theme toggle in the command palette (Ctrl+K) overrides
   it correctly and persists across reloads.
9. **Android build**: with Android Studio / the Android SDK installed,
   open `android/` and confirm it builds (`./gradlew assembleDebug`) —
   not run in this sandbox (no SDK installed, no device/emulator).
10. **Cross-platform binary**: grab the Windows/macOS cross-compiled
    binaries (this sandbox only proved they *compile*, via
    `GOOS=windows/darwin GOARCH=amd64/arm64 go build`) and confirm they
    actually run — the daemon starts, `runyx doctor` reports real
    hardware, and the embedded web UI renders — on real Windows and
    macOS machines.

## After M5 (Android + voice)

11. **Real Android pairing**: run `runyx serve` on a PC and the Android
    app (built from `android/`, e.g. via Android Studio or
    `./gradlew installDebug`) on a real phone on the same LAN. Open the
    PC's pairing QR code (expose `/api/runyx/pairing/qrcode` in a
    browser or the future settings UI), photograph it from the Android
    app's pairing screen, and confirm it connects and lists models.
12. **Real push-to-talk on Android**: with a paired phone, tap the mic
    button, speak, and confirm the transcribed text is sent and a reply
    streams back.
13. **Share Sheet**: from any Android app (browser, notes...), share a
    piece of text and confirm "Runyx" appears as a share target and the
    text lands in the chat input.
14. **mDNS discovery on a real LAN**: confirm `runyx serve`'s mDNS
    announcement is actually discoverable from another machine/phone on
    the same network — this sandbox's network namespace couldn't
    confirm this itself (see docs/BLOCKED.md).
15. **Wake-on-LAN**: with a PC's NIC configured for WoL and the machine
    asleep, send a magic packet to its MAC address and confirm it wakes.
16. **Real whisper.cpp/Piper models**: install `whisper-cli` and `piper`
    on a machine with normal internet access, pull real models, and
    confirm `/api/runyx/voice/transcribe` and `/voice/synthesize` work
    with real audio instead of returning the "not configured" 503 this
    sandbox always saw.
17. **Remote-provider fallback for real**: configure a real OpenAI (or
    other OpenAI-compatible) API key as a remote provider and confirm
    `/api/runyx/remote/chat` actually reaches that service and returns a
    real completion.

This list grows as later milestones add more hardware-dependent
features (plugins, automation, distribution).
