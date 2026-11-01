#!/bin/sh
# Builds the real macOS release artifact: Runyx.app, a minimal but
# genuine app bundle with the native Cocoa/WebKit window compiled in
# (cgo; no cross-compile trickery needed since this is meant to run
# directly on macOS -- a GOARCH matching the host, e.g. CI's macos-14
# (arm64) runner).
#
# Requires: a Go toolchain and Xcode command line tools (clang,
# iconutil, sips -- all preinstalled on GitHub's macos-* runners and
# any normal macOS dev machine).
#
# Usage: scripts/package/macos-release.sh <version> <output-dir>

set -eu

VERSION="${1:?usage: macos-release.sh <version> <output-dir>}"
OUT_DIR="${2:?usage: macos-release.sh <version> <output-dir>}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

mkdir -p "$OUT_DIR"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

# pkgbuild's --version (and Apple's own CFBundleVersion convention)
# expects a dotted-numeric-ish string; a free-form CI version like
# "ci-<sha>" isn't one. Same digit-led stand-in approach as
# linux-release.sh's PKG_VERSION, for the same reason: the package's
# own version metadata needs this, while the binary itself still
# reports the original VERSION via -ldflags/CFBundleVersion below.
case "$VERSION" in
  [0-9]*) PKG_VERSION="$VERSION" ;;
  *) PKG_VERSION="0.0.0" ;;
esac

COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X github.com/furaxdev/runyx/internal/version.Version=$VERSION -X github.com/furaxdev/runyx/internal/version.Commit=$COMMIT -X github.com/furaxdev/runyx/internal/version.BuildDate=$BUILD_DATE"
GOARCH_BUILD="$(go env GOARCH)"

echo "Building Runyx $VERSION ($COMMIT) for darwin/$GOARCH_BUILD with the native Cocoa/WebKit window..."
(
  cd "$REPO_ROOT"
  CGO_ENABLED=1 go build -tags guinative -ldflags "$LDFLAGS" -o "$WORK_DIR/runyx" ./cmd/runyx
)

APP="$WORK_DIR/Runyx.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$WORK_DIR/runyx" "$APP/Contents/MacOS/Runyx"
chmod +x "$APP/Contents/MacOS/Runyx"

# A real .icns, generated the standard macOS way (iconutil needs an
# .iconset directory of specifically-named/sized PNGs) from the single
# 1024px source checked into assets/mac/icon-1024.png.
ICONSET="$WORK_DIR/runyx.iconset"
mkdir -p "$ICONSET"
SRC_ICON="$REPO_ROOT/assets/mac/icon-1024.png"
sips -z 16 16 "$SRC_ICON" --out "$ICONSET/icon_16x16.png" >/dev/null
sips -z 32 32 "$SRC_ICON" --out "$ICONSET/icon_16x16@2x.png" >/dev/null
sips -z 32 32 "$SRC_ICON" --out "$ICONSET/icon_32x32.png" >/dev/null
sips -z 64 64 "$SRC_ICON" --out "$ICONSET/icon_32x32@2x.png" >/dev/null
sips -z 128 128 "$SRC_ICON" --out "$ICONSET/icon_128x128.png" >/dev/null
sips -z 256 256 "$SRC_ICON" --out "$ICONSET/icon_128x128@2x.png" >/dev/null
sips -z 256 256 "$SRC_ICON" --out "$ICONSET/icon_256x256.png" >/dev/null
sips -z 512 512 "$SRC_ICON" --out "$ICONSET/icon_256x256@2x.png" >/dev/null
sips -z 512 512 "$SRC_ICON" --out "$ICONSET/icon_512x512.png" >/dev/null
cp "$SRC_ICON" "$ICONSET/icon_512x512@2x.png"
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/Runyx.icns"

cat > "$APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>Runyx</string>
	<key>CFBundleDisplayName</key>
	<string>Runyx</string>
	<key>CFBundleIdentifier</key>
	<string>com.runyx.app</string>
	<key>CFBundleVersion</key>
	<string>$VERSION</string>
	<key>CFBundleShortVersionString</key>
	<string>$VERSION</string>
	<key>CFBundleExecutable</key>
	<string>Runyx</string>
	<key>CFBundleIconFile</key>
	<string>Runyx</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.developer-tools</string>
</dict>
</plist>
EOF

ARCHIVE_NAME="runyx-darwin-$GOARCH_BUILD.zip"
(cd "$WORK_DIR" && zip -qry "$OUT_DIR/$ARCHIVE_NAME" Runyx.app)

# The real installer: a double-clickable .pkg that runs macOS's own
# Installer.app, installs Runyx.app into /Applications, and needs
# nothing from the user beyond clicking through it. Built in two
# steps, both part of Xcode command line tools (no extra dependency):
# pkgbuild wraps Runyx.app into a plain component package, then
# productbuild wraps *that* into a distribution package with Runyx's
# own branding -- a welcome page (installer-assets-mac/welcome.html)
# and a background image with the mascot
# (installer-assets-mac/background.png, same navy/mascot art the
# Windows installer's wizard banner uses) -- so the installer looks
# like it belongs to Runyx instead of a bare generic Installer.app
# page. Same idea as the Windows Inno Setup installer
# (scripts/package/runyx.iss), platform-native mechanism.
COMPONENT_PKG="$WORK_DIR/component.pkg"
pkgbuild \
  --component "$APP" \
  --install-location /Applications \
  --identifier com.runyx.app \
  --version "$PKG_VERSION" \
  "$COMPONENT_PKG"

DIST_DIR="$WORK_DIR/dist"
mkdir -p "$DIST_DIR"
cp "$REPO_ROOT/scripts/package/installer-assets-mac/background.png" "$DIST_DIR/background.png"
cp "$REPO_ROOT/scripts/package/installer-assets-mac/welcome.html" "$DIST_DIR/welcome.html"
cat > "$DIST_DIR/distribution.xml" <<EOF
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="1">
    <title>Runyx</title>
    <background file="background.png" alignment="topleft" scaling="none"/>
    <welcome file="welcome.html" mime-type="text/html"/>
    <options customize="never" require-scripts="false" rootVolumeOnly="true"/>
    <choices-outline>
        <line choice="default">
            <line choice="com.runyx.app"/>
        </line>
    </choices-outline>
    <choice id="default"/>
    <choice id="com.runyx.app" visible="false">
        <pkg-ref id="com.runyx.app"/>
    </choice>
    <pkg-ref id="com.runyx.app" version="$PKG_VERSION" onConclusion="none">component.pkg</pkg-ref>
</installer-gui-script>
EOF
cp "$COMPONENT_PKG" "$DIST_DIR/component.pkg"

PKG_NAME="Runyx-$PKG_VERSION-Installer-$GOARCH_BUILD.pkg"
productbuild \
  --distribution "$DIST_DIR/distribution.xml" \
  --package-path "$DIST_DIR" \
  --resources "$DIST_DIR" \
  "$OUT_DIR/$PKG_NAME"

(cd "$OUT_DIR" && { sha256sum "$ARCHIVE_NAME" "$PKG_NAME" 2>/dev/null || shasum -a 256 "$ARCHIVE_NAME" "$PKG_NAME"; } >> SHA256SUMS.txt)

echo "Done: $OUT_DIR/$ARCHIVE_NAME, $OUT_DIR/$PKG_NAME"
cat "$OUT_DIR/SHA256SUMS.txt"
