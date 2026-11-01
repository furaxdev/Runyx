#!/bin/sh
# Builds the real Linux release artifact: a `runyx` binary with the
# native WebKitGTK app window compiled in (cgo + GTK3/WebKit2GTK dev
# headers), a .desktop entry + icon set so it shows up as a proper
# application (not just a CLI you remember to launch from a terminal),
# and .deb + Arch packages via nfpm.
#
# Requires: a Go toolchain, libgtk-3-dev, libwebkit2gtk-4.1-dev (or
# -4.0-dev on an older distro that still ships it), pkg-config, and
# nfpm (`go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest`).
#
# Usage: scripts/package/linux-release.sh <version> <output-dir>

set -eu

VERSION="${1:?usage: linux-release.sh <version> <output-dir>}"
OUT_DIR="${2:?usage: linux-release.sh <version> <output-dir>}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

mkdir -p "$OUT_DIR"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

# Distros that renamed the webkit2gtk-4.0 package to webkit2gtk-4.1
# (Ubuntu 24.04+ and recent Debian/Fedora among them) no longer ship a
# webkit2gtk-4.0.pc at all, but webview_go's bundled cgo directives
# still ask for it by that exact name. A pkg-config alias pointing at
# whichever one is actually installed sidesteps needing to patch the
# vendored library. Prefers 4.1 (the current name); falls back to a
# real 4.0 if that's what the system has.
mkdir -p "$WORK_DIR/pkgconfig-shim"
if pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
  REAL_PC="$(pkg-config --variable=pcfiledir webkit2gtk-4.1)/webkit2gtk-4.1.pc"
  sed 's/^Name: .*/Name: WebKit2GTK-4.0-compat-shim/' "$REAL_PC" > "$WORK_DIR/pkgconfig-shim/webkit2gtk-4.0.pc"
elif ! pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
  echo "Neither webkit2gtk-4.1 nor webkit2gtk-4.0 found via pkg-config -- install libwebkit2gtk-4.1-dev (or -4.0-dev)." >&2
  exit 1
fi

# Debian package versions must start with a digit (dpkg hard-rejects
# anything else: "version number does not start with digit"), unlike
# the free-form string that's fine to bake into the binary via
# -ldflags. PKG_VERSION is what nfpm actually packages; VERSION (as
# given) is still what `runyx version` reports, so a CI build's commit
# SHA-based version string ("ci-<sha>") stays visible there even
# though the .deb/Arch package itself needs a digit-led stand-in.
case "$VERSION" in
  [0-9]*) PKG_VERSION="$VERSION" ;;
  *) PKG_VERSION="0.0.0+$VERSION" ;;
esac
# nfpm's deb/archlinux packagers also reject characters outside what
# dpkg/pacman allow in a version string (letters, digits, and . + ~ : -);
# replace anything else (a raw git SHA has none, but this keeps the
# script safe for an arbitrary caller-supplied VERSION too).
PKG_VERSION="$(printf '%s' "$PKG_VERSION" | tr -c 'A-Za-z0-9.+~:-' '_')"

COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X github.com/furaxdev/runyx/internal/version.Version=$VERSION -X github.com/furaxdev/runyx/internal/version.Commit=$COMMIT -X github.com/furaxdev/runyx/internal/version.BuildDate=$BUILD_DATE"

echo "Building runyx $VERSION ($COMMIT) with the native WebKitGTK window..."
(
  cd "$REPO_ROOT"
  CGO_ENABLED=1 \
  PKG_CONFIG_PATH="$WORK_DIR/pkgconfig-shim${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}" \
  go build -tags guinative -ldflags "$LDFLAGS" -o "$WORK_DIR/runyx" ./cmd/runyx
)

PKG_ROOT="$WORK_DIR/pkgroot"
mkdir -p "$PKG_ROOT/usr/bin" "$PKG_ROOT/usr/share/applications" \
  "$PKG_ROOT/usr/share/icons/hicolor/16x16/apps" \
  "$PKG_ROOT/usr/share/icons/hicolor/32x32/apps" \
  "$PKG_ROOT/usr/share/icons/hicolor/48x48/apps" \
  "$PKG_ROOT/usr/share/icons/hicolor/256x256/apps"
cp "$WORK_DIR/runyx" "$PKG_ROOT/usr/bin/runyx"
chmod 0755 "$PKG_ROOT/usr/bin/runyx"
cp "$REPO_ROOT/assets/linux/runyx.desktop" "$PKG_ROOT/usr/share/applications/runyx.desktop"
cp "$REPO_ROOT/assets/linux/icons/runyx-16.png" "$PKG_ROOT/usr/share/icons/hicolor/16x16/apps/runyx.png"
cp "$REPO_ROOT/assets/linux/icons/runyx-32.png" "$PKG_ROOT/usr/share/icons/hicolor/32x32/apps/runyx.png"
cp "$REPO_ROOT/assets/linux/icons/runyx-48.png" "$PKG_ROOT/usr/share/icons/hicolor/48x48/apps/runyx.png"
cp "$REPO_ROOT/assets/linux/icons/runyx-256.png" "$PKG_ROOT/usr/share/icons/hicolor/256x256/apps/runyx.png"

echo "Packaging .deb and Arch Linux packages with nfpm (package version: $PKG_VERSION)..."
# nfpm resolves nfpm.yaml's relative `scripts.postinstall: ./postinstall.sh`
# against the current working directory, not the config file's own
# directory -- cd there first so both nfpm.yaml and postinstall.sh
# resolve correctly regardless of where this script itself was invoked
# from.
(
  cd "$REPO_ROOT/scripts/package"
  NFPM_VERSION="$PKG_VERSION" NFPM_PKGROOT="$PKG_ROOT" \
    nfpm package --config nfpm.yaml --packager deb --target "$OUT_DIR/"
  NFPM_VERSION="$PKG_VERSION" NFPM_PKGROOT="$PKG_ROOT" \
    nfpm package --config nfpm.yaml --packager archlinux --target "$OUT_DIR/"
)

# Also ship a plain tarball (binary + desktop file + icons, same
# layout as the packages above minus the package manager metadata) for
# anyone on a distro without .deb/pacman -- or who just wants to unpack
# it somewhere themselves.
TARBALL_ROOT="$WORK_DIR/tarball/runyx-$VERSION"
mkdir -p "$TARBALL_ROOT"
cp -r "$PKG_ROOT/." "$TARBALL_ROOT/"
cp "$REPO_ROOT/scripts/package/postinstall.sh" "$TARBALL_ROOT/postinstall.sh"
chmod +x "$TARBALL_ROOT/postinstall.sh"
cat > "$TARBALL_ROOT/install.sh" <<'EOF'
#!/bin/sh
# Installs this tarball's contents under / (needs root: run with sudo).
set -eu
cd "$(dirname "$0")"
cp -r usr/* /usr/
./postinstall.sh
EOF
chmod +x "$TARBALL_ROOT/install.sh"
(cd "$WORK_DIR/tarball" && tar -czf "$OUT_DIR/runyx-linux-amd64.tar.gz" "runyx-$VERSION")

(cd "$OUT_DIR" && sha256sum ./*.deb ./*.pkg.tar.zst runyx-linux-amd64.tar.gz > SHA256SUMS.txt 2>/dev/null || sha256sum ./* > SHA256SUMS.txt)

echo "Done:"
ls -la "$OUT_DIR"
cat "$OUT_DIR/SHA256SUMS.txt"
