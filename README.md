# Runyx

**https://tryrunyx.vercel.app**

**Runyx** is a 100% local LLM runner, built to be effortless to install,
pleasant to use, and as fast as raw `llama.cpp`. One binary: CLI, daemon,
and an HTTP server compatible with both the **Ollama** (`/api/*`) and
**OpenAI** (`/v1/*`) APIs, plus an embedded web UI and a companion
Android app. Nothing leaves your machine without your consent.

The product is English-only (UI, CLI, docs) by design — see
`docs/DECISIONS.md`. Voice input still understands whatever language the
user speaks; only the interface itself is English.

## What it does

- **Model catalog** — every entry says what the model is good at, its
  size, and whether it runs well, runs, or is too heavy for your
  machine.
- **Resumable downloads** — an interrupted fetch picks up where it left
  off, and the file is verified by checksum.
- **Ollama-compatible** — import your existing models, keep your tools:
  Runyx answers on the Ollama API *and* the OpenAI API.
- **Every attachment type** — PDF, Word, Excel, PowerPoint, text, code,
  archives. Images go through OCR for models without vision.
- **Knowledge base** — projects with their own instructions, local
  search across your documents with citations, memory you can clear.
- **Private by default** — nothing leaves the machine. No telemetry, no
  account, no tracking.
- **Shell agent** — asks for permission before running anything, stays
  inside the chosen folder, and keeps a log.
- **Phone** — Android pairing by QR code, in text or voice, to the
  models running on your PC.

### Quick start

```bash
go build -o runyx ./cmd/runyx

./runyx doctor                 # hardware detection
./runyx recommend               # models that actually fit this machine
./runyx pull bartowski/Llama-3.2-1B-Instruct-GGUF   # download a model
./runyx run bartowski/Llama-3.2-1B-Instruct-GGUF    # chat from the terminal
./runyx run bartowski/Llama-3.2-1B-Instruct-GGUF --selftest  # one answer-checkable sanity check
./runyx serve                  # start the daemon + HTTP API + web UI
```

Once `runyx serve` is running, these are available on
`http://127.0.0.1:11535`:

- Ollama-compatible: `GET /api/tags`, `POST /api/chat`, `POST /api/generate`, …
- OpenAI-compatible: `GET /v1/models`, `POST /v1/chat/completions`, …

See `docs/ARCHITECTURE.md`, `docs/ROADMAP.md` and `FEATURES.md` for the
rest of the roadmap and current status.

## Status

Actively in development. The screenshots in this repo are real
screenshots.

### License

MIT — see [LICENSE](LICENSE). Third-party dependency licenses are listed
in [THIRD_PARTY.md](THIRD_PARTY.md).
