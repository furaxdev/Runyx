#!/bin/sh
# Rebuilds the web UI and copies it into internal/webui/dist, which is
# go:embed'd into the runyx binary. Run this after changing anything
# under web/src, then commit the updated internal/webui/dist/.
set -eu

cd "$(dirname "$0")/.."

(cd web && npm ci && npm run build)

rm -rf internal/webui/dist
mkdir -p internal/webui/dist
cp -r web/dist/. internal/webui/dist/

echo "UI rebuilt into internal/webui/dist/"
