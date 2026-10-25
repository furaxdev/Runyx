# Runyx — original spec (tracking copy)

> **Note**: the original, verbatim French specification message that
> started this project was never committed to this repository as a
> file — only this amendments record was. The living, maintained
> tracking of every numbered item (including items 25 and 29 referenced
> below) is `FEATURES.md`; this file exists so §1 and those items have
> a place to carry a correction that applies project-wide, as asked.
> If the original full text is needed verbatim, it should be pasted
> back in here.

## §1 — Amendment: UI and docs in English only

**2026-10-22**: the product is English-only (web UI, CLI, Android app,
API error strings, logs, README, `docs/`). See `docs/DECISIONS.md`
("UI and docs in English only") for the reasoning and what was removed
(the FR/EN dict and in-app language toggle) versus kept (the `Lang`
union + per-locale string table mechanism, so a future locale is an
addition, not a rewrite).

## Item 25 — Voice pipeline: amendment

The English-only UI change does **not** extend to speech recognition.
The speech-to-text model continues to understand whatever language the
user actually speaks (French included) — only the interface text around
it (buttons, labels, errors) is English now.

## Item 29 — UI identity: amendment

The UI's identity (theme, layout, components) is unchanged by this
amendment; only its text content moved to English-only, via the single
string table in `web/src/lib/i18n.ts`.
