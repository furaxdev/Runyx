#!/bin/sh
# Generates demo-output/README.md, which demo.yml then copies alongside
# the screenshots both into the workflow artifact and into
# docs/screenshots/ on the demo-screenshots branch. Kept as a separate
# script (not inlined in demo.yml) so it's easy to preview/edit locally:
#   OUT_DIR=demo-output sh scripts/demo/make-readme.sh
set -eu

OUT_DIR="${OUT_DIR:-demo-output}"
README="$OUT_DIR/README.md"

cap() {
  # $1 = theme dir (light|dark), $2 = filename, $3 = caption
  path="$1/$2"
  if [ -f "$OUT_DIR/screenshots/$path" ]; then
    printf '**%s**\n\n![%s](%s)\n\n' "$3" "$3" "$path" >> "$README"
  fi
}

{
  echo "# Runyx — demo screenshots"
  echo
  echo "Generated automatically by [.github/workflows/demo.yml](../../.github/workflows/demo.yml)"
  echo "against a real \`runyx serve\`, a real \`llama-server\` (ggml-org/llama.cpp,"
  echo "CPU build) and a real small model (Qwen2.5-0.5B-Instruct-GGUF) — nothing is"
  echo "mocked. Window fixed at 1440×900 (desktop) and 390×844 (phone), animations"
  echo "disabled for stable captures."
  echo
  echo "A short video of a full conversation (send → streamed reply) is attached to"
  echo "the workflow artifact (not on this branch, to keep it light on every run) —"
  echo "see the repository's Actions tab."
  echo
} > "$README"

for theme_label in "Light theme:light" "Dark theme:dark"; do
  label="${theme_label%%:*}"
  theme="${theme_label##*:}"
  if [ ! -d "$OUT_DIR/screenshots/$theme" ]; then
    continue
  fi
  echo "## $label" >> "$README"
  echo >> "$README"
  cap "$theme" "01-home.png" "Home — greeting, centered composer, shortcuts"
  cap "$theme" "02-command-palette.png" "Command palette (Ctrl+K)"
  cap "$theme" "03-conversation-response.png" "A real conversation, model reply complete"
  cap "$theme" "04-attachment.png" "An attachment added to a message, before sending"
  cap "$theme" "05-models.png" "Models page — Installed / Recommended / Browse all"
  cap "$theme" "06-agent.png" "Agent & permissions center"
  cap "$theme" "07-pairing-qrcode.png" "Mobile pairing — QR code"
  cap "$theme" "08-settings.png" "Settings (appearance)"
  cap "$theme" "09-sidebar-collapsed.png" "Collapsed sidebar, with the device card"
done

if [ -f "$OUT_DIR/screenshots/phone-01-home.png" ]; then
  {
    echo "## Phone format (390×844)"
    echo
    echo "**Home**"
    echo
    echo "![Home, phone format](phone-01-home.png)"
    echo
    echo "**Conversation**"
    echo
    echo "![Conversation, phone format](phone-02-chat.png)"
    echo
  } >> "$README"
fi

echo "README written to $README"
