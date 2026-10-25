package daemon

import (
	"io/fs"
	"net/http"

	"github.com/furaxdev/runyx/internal/webui"
)

// registerStaticUI serves the embedded web UI at "/", falling back to
// index.html for any path that isn't a real asset file (client-side
// routing) — except paths under /api and /v1, which the API routes
// registered before this one already claim.
func registerStaticUI(mux *http.ServeMux) {
	sub, err := fs.Sub(webui.Dist, webui.DistDir)
	if err != nil {
		return // embedded UI wasn't built; API-only mode still works
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("/", spaFallback(sub, fileServer))
}

// spaFallback serves static files when they exist, and index.html
// otherwise, so client-side routes (e.g. a future /settings page) don't
// 404 on a hard refresh.
func spaFallback(fsys fs.FS, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		if _, err := fs.Stat(fsys, path[1:]); err != nil {
			r.URL.Path = "/index.html"
		}
		next.ServeHTTP(w, r)
	})
}
