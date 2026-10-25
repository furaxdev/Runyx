# Third-party dependencies

Runyx itself is MIT-licensed. This lists direct Go module dependencies
and their licenses, checked for compatibility with MIT distribution.

| Module | License | Used for |
|---|---|---|
| [github.com/BurntSushi/toml](https://github.com/BurntSushi/toml) | MIT | Config file (`config.toml`) parsing |
| [golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync) | BSD-3-Clause | `errgroup` for parallel chunked downloads |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | BSD-3-Clause (SQLite core itself is Public Domain) | Local history (conversations, branches, FTS search) — pure Go, no cgo |
| [github.com/google/uuid](https://github.com/google/uuid) | BSD-3-Clause | Message/conversation IDs |
| [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | MIT / Apache-2.0 | Runfile parsing |
| [github.com/ledongthuc/pdf](https://github.com/ledongthuc/pdf) | MIT | PDF text extraction for universal attachments (item 17) |
| [github.com/skip2/go-qrcode](https://github.com/skip2/go-qrcode) | MIT | QR code generation for Android pairing (item 26) |
| [github.com/hashicorp/mdns](https://github.com/hashicorp/mdns) | MIT | mDNS announce/discover for Android pairing (item 26) |
| [github.com/miekg/dns](https://github.com/miekg/dns) (transitive, via hashicorp/mdns) | BSD-3-Clause | DNS message parsing used by the mDNS library |
| [github.com/robfig/cron/v3](https://github.com/robfig/cron) | MIT | Scheduled automation tasks (item 13) |
| [github.com/fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) | BSD-3-Clause | Folder-watch automation (item 13) |

### Android app (`android/`)

| Dependency | License |
|---|---|
| AndroidX (core-ktx, activity-compose, lifecycle-*) | Apache-2.0 |
| Jetpack Compose (ui, material3, compose-bom) | Apache-2.0 |
| androidx.compose.material:material-icons-extended | Apache-2.0 — the icon set used in the app (no emoji) |
| kotlinx-coroutines-android | Apache-2.0 |
| [com.google.zxing:core](https://github.com/zxing/zxing) | Apache-2.0 — QR decoding for pairing (item 26) |
| JUnit 4 (test only) | EPL-1.0 |

## Web UI (`web/`, build-time only — not embedded, only its build output is)

| Package | License |
|---|---|
| svelte | MIT |
| @sveltejs/vite-plugin-svelte | MIT |
| vite | MIT |
| typescript | Apache-2.0 |
| svelte-check | MIT |
| [lucide-svelte](https://github.com/lucide-icons/lucide) | ISC | The icon set used throughout the UI (no emoji) — its SVG icon components ARE bundled into the shipped build output, unlike the rest of this table. |

## Terminal UI (`tui/`)

| Package | License |
|---|---|
| typescript (dev only — typechecking, not bundled) | Apache-2.0 |
| @types/node, bun-types (dev only) | MIT |

No other dependency, runtime or dev: the TUI's rendering engine
(terminal buffer/diff, ANSI/SGR output, input parsing, layout,
widgets, markdown rendering, selection/clipboard) and its daemon
client are all first-party code, by design (see `docs/DECISIONS.md`).
`bun build --compile` produces a single self-contained executable per
platform with nothing else to vendor or license.

No code from Ollama was copied or consulted — the `/api/*` compatibility
layer in `internal/api` is implemented purely from Ollama's public API
documentation (https://github.com/ollama/ollama/blob/main/docs/api.md).

llama.cpp (`llama-server`) is run as an external subprocess, downloaded
from its official GitHub releases (https://github.com/ggml-org/llama.cpp,
MIT license); Runyx does not statically or dynamically link against it.

This file will grow as later milestones add dependencies (Svelte/Vite for
`web/`, whisper.cpp/Piper bindings for voice, etc.) — update it in the
same commit that adds a new dependency.
