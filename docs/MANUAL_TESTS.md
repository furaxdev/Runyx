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

## Windows desktop app (native window, offline use)

Everything in this section needs a real Windows machine: this sandbox
cross-compiled and confirmed the binary (PE32+, GUI subsystem, correct
icon resource) but never ran it, has no Windows to run it on, and has
no WebView2 Runtime to test against. `go test -race ./...` here does
cover `internal/guiwindow`'s fallback-chain logic and `cmd/runyx`'s
single-instance/daemon-reuse logic directly, in isolation — these
items are about the real, integrated, interactive experience.

18. **Double-click opens a window, no console**: download
    `runyx-windows-amd64.zip`, extract it, double-click `Runyx.exe`.
    Confirm: a window opens showing the chat UI (no tabs, no address
    bar), no console/terminal window appears at any point, and the
    taskbar button shows the Runyx mascot icon and "Runyx" as the
    title, not a generic Go/exe icon.
19. **Single instance**: with Runyx already open, double-click
    `Runyx.exe` again. Confirm a second window does *not* open and the
    existing one keeps working.
20. **Closing the window stops the daemon it started**: with Task
    Manager open, confirm a `Runyx.exe` process is running once the
    window is open, then close the window and confirm that process
    exits (no orphaned daemon left running) within a few seconds.
21. **Connects to, rather than duplicates, an existing daemon**: start
    `Runyx.exe serve` from PowerShell first (headless), *then*
    double-click `Runyx.exe`. Confirm the window opens pointed at the
    already-running daemon instead of a second one failing to bind the
    port, and confirm closing the window does *not* kill the `serve`
    process (only a window this process itself started its own daemon
    for should stop that daemon on close).
22. **CLI output still works from a terminal**: from PowerShell, run
    `Runyx.exe doctor` and `Runyx.exe version`. Confirm their output
    prints to that same PowerShell window (proves the
    AttachConsole/CONOUT$ reattachment in `console_windows.go` actually
    works against a real console, not just that it compiles).
23. **WebView2 missing -> falls back to a browser app window**: on a
    machine confirmed *not* to have the WebView2 Runtime installed (a
    fresh Windows Sandbox/VM without it; Windows 11 and recent Windows
    10 ship it by default, so this needs a deliberately bare image),
    double-click `Runyx.exe` and confirm a Microsoft Edge (or Chrome,
    if installed) window opens in `--app=` mode (no tabs/address bar)
    instead of crashing or silently doing nothing.
24. **The actual offline flight-test, end to end**:
    a. On a Windows machine *with* internet, double-click `Runyx.exe`,
       go to the Models page, and download at least one small model
       (e.g. a 1B-class GGUF) to completion.
    b. Turn off Wi-Fi/Ethernet entirely (airplane mode or unplug).
    c. Close Runyx if it's running, then double-click `Runyx.exe`
       again. Confirm the window still opens normally with no network
       connection present.
    d. Select the model you downloaded and send it a message. Confirm
       it loads and replies normally.
    e. Confirm nothing in the UI errors out or hangs waiting on a
       network call that will never complete (fonts, icons and scripts
       are all bundled in `internal/webui/dist`, not fetched from a
       CDN — verified in this sandbox by grepping the built UI for
       external `http(s)://` references, but seeing it actually render
       correctly offline on a real machine is the real test).
25. **Installer wizard, by eye**: download `Runyx-<version>-
    Installer.exe` from the `windows-installer` CI artifact (or build
    it yourself: `iscc /DMyAppVersion=<version> scripts\package\
    runyx.iss` with a built `Runyx.exe` next to it) and actually
    double-click it -- CI already confirmed the silent
    (`/VERYSILENT`) install/uninstall round trip works end to end on
    a real Windows runner (see `scripts/package/README.md`), so what's
    left is purely the interactive wizard: it should run without admin
    rights, create a Start Menu entry, create a desktop shortcut if
    that task is checked, and the "Launch Runyx" checkbox at the end
    should actually open the app window.

## Linux desktop app (Debian/Ubuntu/Arch)

The install-and-smoke-test part of this (items 26-28) was actually
done in this sandbox, for real, with a real `.deb` built by
`scripts/package/linux-release.sh` — see `scripts/package/README.md`
for exactly what that covered (`dpkg -i`, launched under `xvfb-run`,
daemon answered real HTTP requests, single-instance confirmed with two
concurrent launches). What's below is what still needs a human on a
real desktop (a real X11/Wayland session, not a virtual framebuffer)
to see the actual window render and interact with it normally.

26. **App menu entry**: after installing the `.deb` (`sudo dpkg -i
    runyx_*.deb`) or the Arch package (`sudo pacman -U
    runyx-*.pkg.tar.zst`), confirm "Runyx" appears in the desktop
    environment's application launcher/menu with the mascot icon (not
    a generic one), and that launching it from there opens the window
    — not just `runyx` typed into a terminal.
27. **Window actually renders**: on a real desktop session, run
    `runyx` and confirm an actual WebKitGTK window appears with the
    chat UI inside (this sandbox could only confirm the process starts
    and the daemon answers under Xvfb — there is no way to confirm
    pixels actually render correctly without a real display).
28. **Single instance / daemon reuse / closing stops it**: the same
    three checks as the Windows section (items 19-21), redone on a
    real Linux desktop to confirm they hold outside Xvfb too.
29. **Arch package install**: on a real Arch machine, `sudo pacman -U`
    the `.pkg.tar.zst` this sandbox only inspected the contents of
    (never installed, no `pacman` here), and confirm it installs and
    runs the same way the `.deb` was confirmed to.
30. **The offline flight-test**, same as Windows item 24 but on Linux.

## macOS desktop app

`.github/workflows/ci.yml`'s `darwin-release` job builds a real
`Runyx.app` *and* a real `Runyx-<version>-Installer-arm64.pkg` on a
GitHub `macos-14` (Apple Silicon) runner on every push, and actually
installs the `.pkg` with macOS's own `installer -pkg ... -target /`
command-line tool, confirming `Runyx.app` lands in `/Applications` and
runs -- a genuine, automated install round trip. There's no Intel/
`amd64` artifact right now (the `macos-13` runner this used to also
build on sat queued 24h with no runner ever available and was dropped
-- see `docs/DECISIONS.md`); everything below applies equally to an
Intel build someone produces by running `scripts/package/
macos-release.sh` on real Intel hardware. What's below is the
interactive, by-eye part no CI job can check.

31. **Gatekeeper on first launch**: download the `.pkg` (or
    `runyx-darwin-arm64.zip` for the bare `.app`) from the
    `darwin-release` CI artifact and actually run the installer /
    double-click `Runyx.app`. Since it's unsigned and unnotarized,
    macOS will refuse to open it the first time ("cannot be opened
    because the developer cannot be verified") — confirm that System
    Settings → Privacy & Security → "Open Anyway" (or right-click →
    Open) lets it through, and that it then opens normally on every
    later launch.
32. **Installer UI, by eye**: run the `.pkg` interactively (not
    `installer -pkg ... -target /`, which CI already proved works) and
    confirm the standard macOS Installer.app wizard looks right —
    the app name/icon, the install-location step, and that
    `/Applications/Runyx.app` is there and runs afterward.
33. **Window, icon, single instance, daemon reuse**: same checks as
    the Windows/Linux sections, on a real Mac — the window renders
    with no browser chrome, the mascot icon shows in the Dock, a
    second launch doesn't open a duplicate, and closing the window
    stops the daemon it started (or doesn't, if it merely connected to
    one already running).
34. **The offline flight-test**, same as Windows item 24 but on macOS.

This list grows as later milestones add more hardware-dependent
features (plugins, automation, distribution).
