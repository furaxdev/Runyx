#!/bin/sh
# Builds the real macOS release artifact: Runyx.app, a minimal but
# genuine app bundle with the native Cocoa/WebKit window compiled in
# (cgo; no cross-compile trickery needed since this is meant to run
# directly on macOS -- a GOARCH matching the host, e.g. in CI's
# macos-13 (amd64) / macos-14 (arm64) runners).
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

(cd "$OUT_DIR" && sha256sum "$ARCHIVE_NAME" 2>/dev/null >> SHA256SUMS.txt || shasum -a 256 "$ARCHIVE_NAME" >> SHA256SUMS.txt)

echo "Done: $OUT_DIR/$ARCHIVE_NAME"
cat "$OUT_DIR/SHA256SUMS.txt"
