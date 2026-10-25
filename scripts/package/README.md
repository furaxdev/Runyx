# Packaging (item 30)

Status: these are real, usable packaging manifests for each target,
written against Runyx's actual build (`go build ./cmd/runyx`, a single
static binary). **None of them were built/published in this session**
— this sandbox has no Docker daemon, no macOS/brew, no Arch/makepkg, no
`nfpm`, and no winget submission process available to exercise. See
`docs/MANUAL_TESTS.md` for what a maintainer with the right machines
should verify.

| File | Target | Verified here? |
|---|---|---|
| `Dockerfile` | Docker image | No — no Docker daemon in this sandbox |
| `runyx.rb` | Homebrew (macOS/Linux) | No — no brew in this sandbox |
| `PKGBUILD` | Arch/AUR | No — no makepkg in this sandbox |
| `nfpm.yaml` | .deb / .rpm | No — `nfpm` isn't installed here |
| `winget-manifest.yaml` | Windows winget | No — submission is a PR to microsoft/winget-pkgs |
| `windows-release.sh` | Windows `.exe`+zip | Partially — see below |
| `runyx.iss` | Windows installer (Inno Setup) | No — no Windows/Wine/`iscc` in this sandbox |

## Windows desktop app

`windows-release.sh` cross-compiles the real Windows deliverable from
Linux: `Runyx.exe` with a native WebView2 window compiled in (cgo +
`gcc-mingw-w64-x86-64`/`g++-mingw-w64-x86-64`, which this sandbox
installed to prove this), the `-H=windowsgui` linker flag (no console
flash on double-click), the mascot icon embedded as a PE resource
(`cmd/runyx/icon_windows.syso`, generated from
`assets/mascot/svg/runyx-mascot-idle-dark.svg` via `rsvg-convert` +
ImageMagick + `windres`), and the version baked in via `-ldflags`.
Verified in this sandbox: the script runs end to end and produces a
valid PE32+ GUI-subsystem binary with the icon resource present
(checked with `wrestool -l`); `go test -race ./...` passes with
`internal/guiwindow`'s fallback-chain logic and `cmd/runyx`'s
single-instance/daemon-reuse logic unit tested. **Not verified**: the
binary was never actually run, since there's no Windows machine or
Wine here — see `docs/MANUAL_TESTS.md`'s Windows desktop app section
for what a human needs to check on real hardware, including the full
offline flight-test.

`runyx.iss` is an Inno Setup script for an optional installer (Start
Menu entry, optional desktop shortcut, no-admin-required install). Not
built here — needs `iscc` on Windows or under Wine.

## Portable mode

No packaging needed: set `RUNYX_PORTABLE=1` before running the binary
(see `internal/update.PortableDataDir`, unit tested) and all of
Runyx's config/models/backends live in a `runyx-data/` folder next to
the executable — copy that folder plus the binary onto a USB stick and
it runs from there on another machine with the same OS/arch.

## Signed updates

`runyx update` (see `cmd/runyx/cmd_update.go`) checks GitHub for a
newer release and, once a real release signing key exists, verifies an
Ed25519 signature (`internal/update.Verify`, unit tested against real
generated key pairs) before replacing the running binary — with an
automatic `.bak` copy for rollback (`internal/update.Apply`/`Rollback`,
also unit tested). **No real signing key is embedded yet** (`PublicKeyHex`
is empty) — see `docs/DECISIONS.md` for why shipping that empty rather
than fake is the honest choice, and what a real release process needs
to do (generate an Ed25519 key pair, keep the private half secret,
embed `PublicKeyHex` at build time via `-ldflags`, sign each release
asset, publish the signature alongside it).

## AppImage

Not attempted: AppImage packaging needs `appimagetool` and a desktop
entry/icon set this session didn't have time to produce. A static Go
binary is already most of the way there (no runtime deps beyond libc);
the remaining work is purely packaging, not code.
