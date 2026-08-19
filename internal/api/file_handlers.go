package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/sundayprincedev/reader-backend/internal/models"
	"github.com/sundayprincedev/reader-backend/internal/repository"
)

var contentTypes = map[string]string{
	models.FormatPDF:  "application/pdf",
	models.FormatEPUB: "application/epub+zip",
}

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !validKey(key) {
		writeError(w, http.StatusBadRequest, "invalid book key")
		return
	}

	owner := ownerFrom(r.Context())

	book, err := h.books.Get(r.Context(), owner, key)
	if err != nil {
		respondRepositoryError(w, err, "could not load book")
		return
	}
	if book.HasFile {
		writeJSON(w, http.StatusOK, book)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)

	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file must be %d MB or smaller", h.maxUploadBytes/(1<<20)))
		return
	}
	defer r.MultipartForm.RemoveAll()

	upload, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no file provided")
		return
	}
	defer upload.Close()

	if header.Size > h.maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file must be %d MB or smaller", h.maxUploadBytes/(1<<20)))
		return
	}

	fileID, err := h.files.Store(r.Context(), key, upload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store file")
		return
	}

	updated, err := h.books.AttachFile(r.Context(), owner, key, fileID)
	if err != nil {
		_ = h.files.Delete(r.Context(), fileID)
		respondRepositoryError(w, err, "could not attach file")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
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
	if !book.HasFile || book.FileID == nil {
		writeError(w, http.StatusNotFound, "no file stored for this book")
		return
	}

	etag := `"` + book.FileID.Hex() + `"`
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", contentTypes[book.Format])
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", book.SizeBytes))

	if err := h.files.Stream(r.Context(), *book.FileID, w); err != nil && !errors.Is(err, repository.ErrNotFound) {
		return
	}
}
