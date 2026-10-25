# Packaging (item 30)

Status: these are real, usable packaging manifests for each target,
written against Runyx's actual build (`go build ./cmd/runyx`). See
`docs/MANUAL_TESTS.md` for what a maintainer with the right machines
should still verify by hand.

| File | Target | Verified here? |
|---|---|---|
| `windows-release.sh` | Windows `Runyx.exe`+zip | Build verified (sandbox cross-compile + a real `windows-release` GitHub Actions run); never actually run — see below |
| `linux-release.sh` + `nfpm.yaml` | `.deb` / Arch Linux package / tarball | **Yes** — built, installed via `dpkg -i`, and actually launched under Xvfb on this sandbox; see below |
| `PKGBUILD` | Arch/AUR (from-source build) | No `makepkg`/Arch machine here, but builds the exact same `-tags guinative` binary the nfpm Arch package above does, which *was* run |
| `macos-release.sh` | macOS `Runyx.app`+zip | Not run in this sandbox (no macOS); runs for real in CI's `darwin-release` job on GitHub's own `macos-13`/`macos-14` runners — see below |
| `runyx.rb` | Homebrew (macOS/Linux) | No — no brew in this sandbox |
| `nfpm.yaml` (.rpm) | `.rpm` | No — not built here, same tool as the verified `.deb`/Arch targets above so likely to work, but unverified |
| `winget-manifest.yaml` | Windows winget | No — submission is a PR to microsoft/winget-pkgs |
| `Dockerfile` | Docker image | No — no Docker daemon in this sandbox |
| `runyx.iss` | Windows installer (Inno Setup) | No — no Windows/Wine/`iscc` in this sandbox |

## Linux desktop app

`linux-release.sh` builds `runyx` with the native WebKitGTK app window
compiled in (`-tags guinative`, cgo, `libgtk-3-dev`/
`libwebkit2gtk-4.1-dev`), a `.desktop` entry (`assets/linux/
runyx.desktop`) and icon set (`assets/linux/icons/`, rasterized from
the mascot SVG), then packages all of it with `nfpm` into a real
`.deb`, a real Arch Linux `.pkg.tar.zst`, and a plain tarball.

**Actually verified in this sandbox, not just built**: installed the
built `.deb` with `dpkg -i` on this Debian-based system, confirmed
`runyx version` runs from `/usr/bin/runyx`, confirmed the `.desktop`
file and all 4 icon sizes landed under `/usr/share/`, then launched
`runyx` for real under `xvfb-run` (a virtual X display, since this
sandbox has no real one) — the GTK/WebKit process started, stayed up
(not a crash loop), and its embedded HTTP daemon answered real
requests on `127.0.0.1:11535`. Also launched it a second time while
the first was still running and confirmed the single-instance logic
correctly printed "Runyx is already running" and exited instead of
opening a duplicate. Cleaned up (`dpkg -r runyx`) afterward. The Arch
package was inspected (`.PKGINFO`, file layout) but not installed,
since this sandbox has no `pacman`. `.github/workflows/ci.yml`'s
`linux-release` job repeats this same install-and-smoke-test on every
push, for real, on a fresh `ubuntu-latest` runner.

Both `nfpm.yaml` and `PKGBUILD` need `${NFPM_...}`-style env expansion
or (for `PKGBUILD`) an inline `pkg-config` alias to work around the
same issue `windows-release.sh` hits with a differently-named header:
current Ubuntu/Debian/Arch all rename the `webkit2gtk-4.0` package to
`webkit2gtk-4.1`, but `webview_go`'s bundled cgo directives still ask
for `webkit2gtk-4.0` by that exact name — worked around with a
one-line pkg-config shim rather than patching the vendored dependency.

## Windows desktop app

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

## macOS desktop app

`macos-release.sh` builds a real `Runyx.app` bundle: the native
Cocoa/WebKit window compiled in (`-tags guinative`, cgo -- no cross
compiler needed or possible for this combination, since a cgo build
targeting Cocoa only works compiled natively on macOS), a real
`.icns` generated the standard way (`sips` + `iconutil`) from
`assets/mac/icon-1024.png` (the mascot SVG rasterized at 1024px), and
an `Info.plist` with the bundle identifier/version/icon wired up.

Not run in this sandbox at all -- there's no macOS here, and unlike
Windows' mingw-w64 there is no practical way to cross-compile a cgo
Cocoa/WebKit binary from Linux (would need a full macOS SDK + osxcross
setup, fragile and not attempted). Instead,
`.github/workflows/ci.yml`'s `darwin-release` job runs this script on
two of GitHub's own macOS runners (`macos-13`, an Intel machine, and
`macos-14`, Apple Silicon), each building and uploading its native
`Runyx.app` as a real CI artifact -- genuine macOS hardware/VMs doing
the actual build, the strongest verification available without a Mac
in hand. See `docs/MANUAL_TESTS.md` for what a human still needs to
check by actually opening the built `.app` (first-launch Gatekeeper
prompt since it's unsigned/unnotarized, the window itself, offline
behavior).

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
