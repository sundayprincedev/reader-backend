package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/sundayprincedev/reader-backend/internal/models"
	"github.com/sundayprincedev/reader-backend/internal/repository"
)

var contentTypes = map[string]string{
	models.FormatPDF:  "application/pdf",
	models.FormatEPUB: "application/epub+zip",
}

var magicNumbers = map[string][]byte{
	models.FormatPDF:  []byte("%PDF-"),
	models.FormatEPUB: {'P', 'K', 0x03, 0x04},
}

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !validKey(key) {
		writeError(w, http.StatusBadRequest, "invalid book key")
		return
	}

	book, err := h.books.Get(r.Context(), key)
	if err != nil {
		respondRepositoryError(w, err, "could not load book")
		return
	}
	if book.HasFile {
		writeJSON(w, http.StatusOK, book)
		return
	}

	limit := fmt.Sprintf("file must be %d MB or smaller", h.maxUploadBytes/(1<<20))
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)

	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, limit)
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
		writeError(w, http.StatusRequestEntityTooLarge, limit)
		return
	}

	if err := verifyMagicNumber(upload, book.Format); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	fileID, err := h.files.Store(r.Context(), key, upload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store file")
		return
	}

	updated, err := h.books.AttachFile(r.Context(), key, fileID, header.Size)
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

	book, err := h.books.Get(r.Context(), key)
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

	if err := h.files.Stream(r.Context(), *book.FileID, w); err != nil && !errors.Is(err, repository.ErrNotFound) {
		return
	}
}

func verifyMagicNumber(file io.ReadSeeker, format string) error {
	expected, known := magicNumbers[format]
	if !known {
		return errors.New("unsupported format")
	}

	header := make([]byte, len(expected))
	if _, err := io.ReadFull(file, header); err != nil {
		return errors.New("file is empty or unreadable")
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return errors.New("file could not be read")
	}

	if !bytes.Equal(header, expected) {
		return fmt.Errorf("that file is not a valid %s", format)
	}
	return nil
}
