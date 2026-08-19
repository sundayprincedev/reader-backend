package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sundayprincedev/reader-backend/internal/auth"
)

type Options struct {
	Books          *Handler
	Auth           *AuthHandler
	Issuer         *auth.Issuer
	AllowedOrigins []string
	Timeout        time.Duration
	StaticDir      string
}

func NewRouter(opts Options) http.Handler {
	guard := withAuth(opts.Issuer)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", opts.Books.Health)
	mux.HandleFunc("POST /api/auth/register", opts.Auth.Register)
	mux.HandleFunc("POST /api/auth/login", opts.Auth.Login)
	mux.Handle("GET /api/auth/me", guard(http.HandlerFunc(opts.Auth.Me)))

	mux.Handle("GET /api/books", guard(http.HandlerFunc(opts.Books.ListBooks)))
	mux.Handle("POST /api/books", guard(http.HandlerFunc(opts.Books.RegisterBook)))
	mux.Handle("GET /api/books/{key}", guard(http.HandlerFunc(opts.Books.GetBook)))
	mux.Handle("DELETE /api/books/{key}", guard(http.HandlerFunc(opts.Books.DeleteBook)))
	mux.Handle("PUT /api/books/{key}/progress", guard(http.HandlerFunc(opts.Books.SaveProgress)))
	mux.Handle("POST /api/books/{key}/reset", guard(http.HandlerFunc(opts.Books.ResetProgress)))
	mux.Handle("POST /api/books/{key}/restore", guard(http.HandlerFunc(opts.Books.RestoreProgress)))
	mux.Handle("POST /api/books/{key}/file", guard(http.HandlerFunc(opts.Books.UploadFile)))
	mux.Handle("GET /api/books/{key}/file", guard(http.HandlerFunc(opts.Books.DownloadFile)))

	root := http.NewServeMux()
	root.Handle("/api/", mux)
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
