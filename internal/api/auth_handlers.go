package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/sundayprincedev/reader-backend/internal/auth"
	"github.com/sundayprincedev/reader-backend/internal/models"
	"github.com/sundayprincedev/reader-backend/internal/repository"
)

const minPasswordLength = 8

type AuthHandler struct {
	users  *repository.UserRepository
	issuer *auth.Issuer
}

func NewAuthHandler(users *repository.UserRepository, issuer *auth.Issuer) *AuthHandler {
	return &AuthHandler{users: users, issuer: issuer}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var credentials models.Credentials
	if !decode(w, r, &credentials) {
		return
	}

	email := repository.NormalizeEmail(credentials.Email)
	if !validEmail(email) {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	if len(credentials.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(credentials.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	user, err := h.users.Create(r.Context(), email, hash)
	if errors.Is(err, repository.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "an account with this email already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	h.respondWithToken(w, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var credentials models.Credentials
	if !decode(w, r, &credentials) {
		return
	}

	user, err := h.users.ByEmail(r.Context(), credentials.Email)
	if err != nil || !auth.VerifyPassword(user.PasswordHash, credentials.Password) {
		writeError(w, http.StatusUnauthorized, "email or password is incorrect")
		return
	}

	h.respondWithToken(w, user)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.users.ByID(r.Context(), ownerFrom(r.Context()))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session expired")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) respondWithToken(w http.ResponseWriter, user models.User) {
	token, err := h.issuer.Sign(user.ID.Hex())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}

	writeJSON(w, http.StatusOK, models.AuthResponse{Token: token, User: user})
}

func validEmail(email string) bool {
	if len(email) < 5 || len(email) > 254 {
		return false
	}

	at := strings.IndexByte(email, '@')
	if at < 1 || at != strings.LastIndexByte(email, '@') {
		return false
	}

	domain := email[at+1:]
	dot := strings.LastIndexByte(domain, '.')
	return dot > 0 && dot < len(domain)-1 && !strings.ContainsAny(email, " \t\r\n")
}
