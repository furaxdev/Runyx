# Packaging (item 30)

Status: these are real, usable packaging manifests for each target,
written against Runyx's actual build (`go build ./cmd/runyx`). See
`docs/MANUAL_TESTS.md` for what a maintainer with the right machines
should still verify by hand.

| File | Target | Verified here? |
|---|---|---|
| `windows-release.sh` | Windows `Runyx.exe`+zip | Build verified (sandbox cross-compile + a real `windows-release` GitHub Actions run); never actually run on Windows itself |
| `runyx.iss` | Windows installer (`Runyx-<version>-Installer.exe`, Inno Setup) | **Yes** — built, silently installed, launched, and silently uninstalled for real on a native `windows-latest` GitHub Actions runner (`windows-installer` job); never run on this sandbox (no Windows/Wine here) — see below |
| `linux-release.sh` + `nfpm.yaml` | `.deb` / Arch Linux package / tarball | **Yes** — built, installed via `dpkg -i`, and actually launched under Xvfb on this sandbox; see below |
| `PKGBUILD` | Arch/AUR (from-source build) | No `makepkg`/Arch machine here, but builds the exact same `-tags guinative` binary the nfpm Arch package above does, which *was* run |
| `macos-release.sh` | macOS `Runyx.app`+zip, `Runyx-<version>-Installer-arm64.pkg` | Not run in this sandbox (no macOS); the `.app` *and* the `.pkg` installer are both built and the `.pkg` is actually installed via `installer -pkg` for real in CI's `darwin-release` job, on a GitHub `macos-14` (Apple Silicon) runner — see below. No Intel/`amd64` leg right now: `macos-13` sat queued 24h with no runner ever available and was dropped rather than guessed at again |
| `runyx.rb` | Homebrew (macOS/Linux) | No — no brew in this sandbox |
| `nfpm.yaml` (.rpm) | `.rpm` | No — not built here, same tool as the verified `.deb`/Arch targets above so likely to work, but unverified |
| `winget-manifest.yaml` | Windows winget | No — submission is a PR to microsoft/winget-pkgs |
| `Dockerfile` | Docker image | No — no Docker daemon in this sandbox |

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

`windows-release.sh` cross-compiles `Runyx.exe` from Linux: a native
WebView2 window compiled in (cgo + `gcc-mingw-w64-x86-64`/
`g++-mingw-w64-x86-64`, which this sandbox installed to prove this),
the `-H=windowsgui` linker flag (no console flash on double-click), the
mascot icon embedded as a PE resource (`cmd/runyx/icon_windows.syso`,
generated from `assets/mascot/svg/runyx-mascot-idle-dark.svg` via
`rsvg-convert` + ImageMagick + `windres`), and the version baked in via
`-ldflags`. Verified in this sandbox: the script runs end to end and
produces a valid PE32+ GUI-subsystem binary with the icon resource
present (checked with `wrestool -l`).

`runyx.iss` (Inno Setup) is the real installer -- `Runyx-<version>-
Installer.exe`, which a user double-clicks, clicks through a short
wizard (Start Menu entry, optional desktop shortcut, no admin rights
required since Runyx is portable-by-default), and ends with Runyx
installed and optionally launched. **This is actually built and
verified, just not in this sandbox**: `.github/workflows/ci.yml`'s
`windows-installer` job runs on a native `windows-latest` GitHub
Actions runner (not a cross-compile -- real Windows, mingw-w64 and
Inno Setup installed via Chocolatey), builds `Runyx.exe` there
directly, compiles `runyx.iss` with `ISCC.exe` into the real installer
executable, then **silently installs it** (`/VERYSILENT`), confirms
`Runyx.exe` landed where Inno Setup put it and actually runs
(`Runyx.exe version`), **silently uninstalls it**, and confirms it's
gone -- a full real install/uninstall round trip on genuine Windows,
automated, on every push. Not run interactively by a human anywhere
yet -- see `docs/MANUAL_TESTS.md`'s Windows desktop app section for
what still needs eyes on a real screen (the wizard UI itself, the
Start Menu/desktop shortcuts actually opening the app, the full
offline flight-test).

## macOS desktop app

`macos-release.sh` builds a real `Runyx.app` bundle: the native
Cocoa/WebKit window compiled in (`-tags guinative`, cgo -- no cross
compiler needed or possible for this combination, since a cgo build
targeting Cocoa only works compiled natively on macOS), a real
`.icns` generated the standard way (`sips` + `iconutil`) from
`assets/mac/icon-1024.png` (the mascot SVG rasterized at 1024px), and
an `Info.plist` with the bundle identifier/version/icon wired up. It
then wraps that `.app` into **the real installer**: a `Runyx-<version>-
Installer-<arch>.pkg`, built with `pkgbuild` (Xcode command line
tools, no extra dependency), targeting `/Applications` -- a user
double-clicks it, macOS's own Installer.app runs, and Runyx ends up
installed, same shape as the Windows Inno Setup installer above.

Not run in this sandbox at all -- there's no macOS here, and unlike
Windows' mingw-w64 there is no practical way to cross-compile a cgo
Cocoa/WebKit binary from Linux (would need a full macOS SDK + osxcross
setup, fragile and not attempted). Instead,
`.github/workflows/ci.yml`'s `darwin-release` job runs this script on
a GitHub `macos-14` (Apple Silicon) runner, building the `.app` and
the `.pkg` natively, then **actually installs the `.pkg`** with
macOS's own `installer -pkg ... -target /` command-line tool, confirms
`Runyx.app` landed in `/Applications` and runs (`Runyx version`), and
removes it again -- a real install round trip on genuine macOS
hardware/VMs, automated, on every push; the strongest verification
available without a Mac in hand. **No Intel/`amd64` build right now**:
the matrix originally also had a `macos-13` (Intel) leg, but that
runner label sat queued 24 hours with no runner ever picked up on this
account and was dropped rather than guessed at again -- the script
itself still builds whatever `go env GOARCH` is on the machine running
it, so an Intel Mac (or a working Intel-capable runner label) can
still produce one directly. See `docs/MANUAL_TESTS.md` for what a
human still needs to check by actually opening the installed `.app`
(first-launch Gatekeeper prompt since it's unsigned/unnotarized, the
window itself, offline behavior).

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
