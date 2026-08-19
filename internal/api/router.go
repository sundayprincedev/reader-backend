package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	Books          *Handler
	AccessPIN      string
	AllowedOrigins []string
	Timeout        time.Duration
	StaticDir      string
}

func NewRouter(opts Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/unlock", opts.Books.Unlock)
	mux.HandleFunc("GET /api/books", opts.Books.ListBooks)
	mux.HandleFunc("POST /api/books", opts.Books.RegisterBook)
	mux.HandleFunc("GET /api/books/{key}", opts.Books.GetBook)
	mux.HandleFunc("DELETE /api/books/{key}", opts.Books.DeleteBook)
	mux.HandleFunc("PUT /api/books/{key}/progress", opts.Books.SaveProgress)
	mux.HandleFunc("POST /api/books/{key}/reset", opts.Books.ResetProgress)
	mux.HandleFunc("POST /api/books/{key}/restore", opts.Books.RestoreProgress)
	mux.HandleFunc("POST /api/books/{key}/file", opts.Books.UploadFile)
	mux.HandleFunc("GET /api/books/{key}/file", opts.Books.DownloadFile)

	root := http.NewServeMux()
	root.HandleFunc("GET /api/health", opts.Books.Health)
	root.Handle("/api/", withPIN(opts.AccessPIN)(mux))
	root.Handle("/", spaHandler(opts.StaticDir))

	return chain(root,
		withRecovery,
		withLogging,
		withCORS(opts.AllowedOrigins),
		withTimeout(opts.Timeout),
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
