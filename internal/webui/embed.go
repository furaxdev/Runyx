// Package webui embeds the built web UI (web/ — Svelte + Vite) into the
// runyx binary via go:embed, so the daemon can serve it without any
// runtime dependency on Node/npm.
//
// dist/ is produced by `scripts/build-ui.sh` (runs `npm ci && npm run
// build` in web/ and copies the result here) and committed so that a
// plain `go build ./cmd/runyx` always produces a binary with a working
// UI, even without Node installed. Run scripts/build-ui.sh again after
// changing anything under web/src and commit the updated dist/.
package webui

import "embed"

//go:embed dist
var Dist embed.FS

// DistDir is the subdirectory within Dist holding the actual files,
// matching how embed.FS roots its paths at the directive's pattern.
const DistDir = "dist"
