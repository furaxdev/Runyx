#!/bin/sh
# Fails if an emoji appears anywhere in source or docs (the product
# rule: icons/ASCII, never emoji — see docs/DECISIONS.md). Checked in
# CI on every push/PR.
set -eu

cd "$(dirname "$0")/.."

# Emoji ranges: pictographs/symbols/transport/supplemental symbols
# (1F300-1FAFF), mahjong/dominoes/playing cards (1F000-1F2FF), and misc
# symbols/dingbats (2600-27BF) — the last of which also covers glyphs
# like checkmarks/stars some earlier UI code used in place of real
# icons; those should be real icon components now, not characters.
PATTERN='[\x{1F300}-\x{1FAFF}\x{1F000}-\x{1F2FF}\x{2600}-\x{27BF}]'

hits=$(LC_ALL=C.UTF-8 grep -rnP --binary-files=text "$PATTERN" \
  web/src cmd internal android/app/src tui/src tui/test docs README.md FEATURES.md RUNYX_PROMPT.md THIRD_PARTY.md models \
  --exclude-dir=node_modules --exclude-dir=dist --exclude-dir=build --exclude-dir=dist-tui --exclude-dir=dist-ts \
  2>/dev/null || true)

if [ -n "$hits" ]; then
  echo "Emoji/pictograph characters found (use the icon set or plain text instead):"
  echo "$hits"
  exit 1
fi

echo "No emoji found."
