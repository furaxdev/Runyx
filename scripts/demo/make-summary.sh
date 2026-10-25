#!/bin/sh
# Writes a GitHub Actions Job Summary: the CLI transcripts in full, plus a
# couple of screenshots pulled straight from the just-pushed
# demo-screenshots branch (so they render inline without needing the
# artifact download). Printed to stdout; demo.yml redirects it into
# $GITHUB_STEP_SUMMARY.
set -eu

OUT_DIR="${OUT_DIR:-demo-output}"
REPO="${GITHUB_REPOSITORY:-furaxdev/Runyx}"
RAW_BASE="https://raw.githubusercontent.com/$REPO/demo-screenshots/docs/screenshots"

echo "# Runyx demo"
echo
if [ -f "$OUT_DIR/screenshots/light/03-conversation-response.png" ]; then
  echo "![conversation](${RAW_BASE}/light/03-conversation-response.png)"
  echo
fi
echo "All screenshots: [docs/screenshots on the \`demo-screenshots\` branch](https://github.com/$REPO/tree/demo-screenshots/docs/screenshots)"
echo

for f in "$OUT_DIR"/terminal/*.txt; do
  [ -f "$f" ] || continue
  name=$(basename "$f")
  echo "<details><summary><code>$name</code></summary>"
  echo
  echo '```'
  cat "$f"
  echo '```'
  echo
  echo "</details>"
  echo
done
