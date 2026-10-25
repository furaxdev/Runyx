#!/bin/sh
# Copies the editable models/registry.json into internal/registry/, which
# go:embed's it into the runyx binary. Run this after editing
# models/registry.json, then commit the updated internal/registry/registry.json.
set -eu

cd "$(dirname "$0")/.."

cp models/registry.json internal/registry/registry.json

echo "registry synced into internal/registry/registry.json"
