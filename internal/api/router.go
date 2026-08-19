package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func NewRouter(handler *Handler, allowedOrigins []string, timeout time.Duration, staticDir string) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/health", handler.Health)
	api.Handle("GET /api/books", withOwner(http.HandlerFunc(handler.ListBooks)))
	api.Handle("POST /api/books", withOwner(http.HandlerFunc(handler.RegisterBook)))
	api.Handle("GET /api/books/{key}", withOwner(http.HandlerFunc(handler.GetBook)))
	api.Handle("DELETE /api/books/{key}", withOwner(http.HandlerFunc(handler.DeleteBook)))
	api.Handle("PUT /api/books/{key}/progress", withOwner(http.HandlerFunc(handler.SaveProgress)))
	api.Handle("POST /api/books/{key}/reset", withOwner(http.HandlerFunc(handler.ResetProgress)))
	api.Handle("POST /api/books/{key}/restore", withOwner(http.HandlerFunc(handler.RestoreProgress)))

	root := http.NewServeMux()
	root.Handle("/api/", api)
	root.Handle("/", spaHandler(staticDir))

	return chain(root,
		withRecovery,
		withLogging,
		withCORS(allowedOrigins),
		withTimeout(timeout),
	)
}

func spaHandler(staticDir string) http.Handler {
	if staticDir == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not found")
		})
	}

	fileServer := http.FileServer(http.Dir(staticDir))
	indexPath := filepath.Join(staticDir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := filepath.Join(staticDir, filepath.Clean(r.URL.Path))

		if info, err := os.Stat(requested); err == nil && !info.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, indexPath)
	})
}
