package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sundayprincedev/reader-backend/internal/models"
	"github.com/sundayprincedev/reader-backend/internal/repository"
)

const maxBodyBytes = 1 << 16

type Handler struct {
	books          *repository.BookRepository
	files          *repository.FileRepository
	maxUploadBytes int64
}

func NewHandler(books *repository.BookRepository, files *repository.FileRepository, maxUploadBytes int64) *Handler {
	return &Handler{books: books, files: files, maxUploadBytes: maxUploadBytes}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ListBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.books.List(r.Context(), ownerFrom(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load library")
		return
	}

	writeJSON(w, http.StatusOK, models.LibraryResponse{Books: books, Stats: buildStats(books)})
}

func (h *Handler) RegisterBook(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if !decode(w, r, &req) {
		return
	}

	req.Key = strings.TrimSpace(req.Key)
	req.Title = strings.TrimSpace(req.Title)
	req.Author = strings.TrimSpace(req.Author)
	req.Format = strings.ToLower(strings.TrimSpace(req.Format))

	if !validKey(req.Key) {
		writeError(w, http.StatusBadRequest, "invalid book key")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Format != models.FormatPDF && req.Format != models.FormatEPUB {
		writeError(w, http.StatusBadRequest, "format must be pdf or epub")
		return
	}

	book, err := h.books.Register(r.Context(), ownerFrom(r.Context()), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not register book")
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !validKey(key) {
		writeError(w, http.StatusBadRequest, "invalid book key")
		return
	}

	book, err := h.books.Get(r.Context(), ownerFrom(r.Context()), key)
	if err != nil {
		respondRepositoryError(w, err, "could not load book")
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (h *Handler) SaveProgress(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !validKey(key) {
		writeError(w, http.StatusBadRequest, "invalid book key")
		return
	}

	var req models.ProgressRequest
	if !decode(w, r, &req) {
		return
	}

	book, err := h.books.SaveProgress(r.Context(), ownerFrom(r.Context()), key, req)
	if err != nil {
		respondRepositoryError(w, err, "could not save progress")
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (h *Handler) ResetProgress(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !validKey(key) {
		writeError(w, http.StatusBadRequest, "invalid book key")
		return
	}

	book, err := h.books.Reset(r.Context(), ownerFrom(r.Context()), key)
	if err != nil {
		respondRepositoryError(w, err, "could not reset progress")
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (h *Handler) RestoreProgress(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !validKey(key) {
		writeError(w, http.StatusBadRequest, "invalid book key")
		return
	}

	var req struct {
		Index int `json:"index"`
	}
	if !decode(w, r, &req) {
		return
	}

	book, err := h.books.Restore(r.Context(), ownerFrom(r.Context()), key, req.Index)
	if err != nil {
		respondRepositoryError(w, err, "could not restore checkpoint")
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (h *Handler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !validKey(key) {
		writeError(w, http.StatusBadRequest, "invalid book key")
		return
	}

	owner := ownerFrom(r.Context())

	book, err := h.books.Get(r.Context(), owner, key)
	if err != nil {
		respondRepositoryError(w, err, "could not remove book")
		return
	}

	if err := h.books.Delete(r.Context(), owner, key); err != nil {
		respondRepositoryError(w, err, "could not remove book")
		return
	}

	if book.FileID != nil {
		_ = h.files.Delete(r.Context(), *book.FileID)
	}

	w.WriteHeader(http.StatusNoContent)
}

func buildStats(books []models.Book) models.LibraryStats {
	stats := models.LibraryStats{Books: len(books)}

	total := 0.0
	for _, book := range books {
		total += book.Current.Percent
		stats.SecondsRead += book.SecondsRead
		if book.Finished {
			stats.Finished++
			continue
		}
		if book.Current.Percent > 0 {
			stats.Started++
		}
	}

	if len(books) > 0 {
		stats.AveragePct = total / float64(len(books))
	}
	return stats
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func validKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for _, char := range key {
		isHex := (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')
		if !isHex {
			return false
		}
	}
	return true
}

func respondRepositoryError(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}
	writeError(w, http.StatusInternalServerError, message)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
