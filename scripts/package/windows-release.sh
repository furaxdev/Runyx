#!/bin/sh
# Builds the real Windows release artifact: a single Runyx.exe with the
# native WebView2 app window compiled in (CGO + mingw-w64), a Windows
# subsystem that shows no console on double-click, the mascot icon
# embedded as a resource, and the version baked in via -ldflags. Then
# packages it with a short README into a zip and writes its checksum,
# matching what `runyx.exe` alone needs to run -- no DLLs, no installer
# required.
#
# Requires, on the build machine (or installed by CI before calling
# this): a Go toolchain, and the mingw-w64 cross compiler
# (gcc-mingw-w64-x86-64, g++-mingw-w64-x86-64) if cross-compiling from
# Linux/macOS, or just a regular MSVC/mingw toolchain if run on
# Windows itself with GOOS/GOARCH left at their native values.
#
# Usage: scripts/package/windows-release.sh <version> <output-dir>
# Example: scripts/package/windows-release.sh v0.1.0 /tmp/release

set -eu

VERSION="${1:?usage: windows-release.sh <version> <output-dir>}"
OUT_DIR="${2:?usage: windows-release.sh <version> <output-dir>}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

mkdir -p "$OUT_DIR"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

# mingw-w64's bundled Windows SDK headers spell this file lowercase
# (eventtoken.h) while WebView2.h #includes it as "EventToken.h"; on a
# case-sensitive filesystem that's a hard build failure unless a
# correctly-cased shim is on the include path ahead of the real one.
mkdir -p "$WORK_DIR/mingw-shim"
echo '#include "eventtoken.h"' > "$WORK_DIR/mingw-shim/EventToken.h"

COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-H=windowsgui -X github.com/furaxdev/runyx/internal/version.Version=$VERSION -X github.com/furaxdev/runyx/internal/version.Commit=$COMMIT -X github.com/furaxdev/runyx/internal/version.BuildDate=$BUILD_DATE"

echo "Building Runyx.exe $VERSION ($COMMIT)..."
(
  cd "$REPO_ROOT"
  CGO_ENABLED=1 \
  GOOS=windows GOARCH=amd64 \
  CC=x86_64-w64-mingw32-gcc \
  CXX=x86_64-w64-mingw32-g++ \
  CGO_CFLAGS="-I$WORK_DIR/mingw-shim" \
  CGO_CXXFLAGS="-I$WORK_DIR/mingw-shim" \
  go build -ldflags "$LDFLAGS" -o "$WORK_DIR/Runyx.exe" ./cmd/runyx
)

cat > "$WORK_DIR/README.txt" <<EOF
Runyx $VERSION for Windows
===========================

Double-click Runyx.exe. A window opens with the chat interface --
nothing else to install or configure. The first time you ask it to
load a model, it downloads the llama.cpp backend it needs (one-time,
needs internet); after that, and after you've downloaded at least one
model with the "Models" page, Runyx works fully offline.

If no window appears and nothing seems to happen: Runyx may already
be running (look for it in the system tray / task manager) -- only
one instance runs at a time by design, and a second double-click
just connects to the one already open rather than starting another.

Command-line use (optional): open Runyx.exe from PowerShell or cmd.exe
for the full CLI (runyx doctor, runyx pull <model>, runyx serve for a
windowless daemon, etc.) -- see https://github.com/furaxdev/Runyx.

Verify this download against SHA256SUMS.txt (from PowerShell):
  Get-FileHash Runyx.exe -Algorithm SHA256
EOF

ZIP_NAME="runyx-windows-amd64.zip"
(cd "$WORK_DIR" && zip -q "$OUT_DIR/$ZIP_NAME" Runyx.exe README.txt)

(cd "$OUT_DIR" && sha256sum "$ZIP_NAME" > SHA256SUMS.txt)

echo "Done: $OUT_DIR/$ZIP_NAME"
cat "$OUT_DIR/SHA256SUMS.txt"
