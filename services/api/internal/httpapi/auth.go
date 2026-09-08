package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"personalvpn/services/api/internal/auth"
	"personalvpn/services/api/internal/control"
)

const maxJSONBody = 16 * 1024

type AuthService interface {
	Register(ctx context.Context, email, password string) (string, error)
	Login(ctx context.Context, email, password string) (auth.TokenPair, error)
	Refresh(refreshToken string) (auth.TokenPair, string, error)
	Logout(userID string)
	AuthenticateAccess(accessToken string) (string, error)
}

type AuthHandler struct {
	service AuthService
}

func NewAuthHandler(service AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	userID, err := h.service.Register(r.Context(), request.Email, request.Password)
	if err != nil {
		if errors.Is(err, control.ErrConflict) {
			writeProblem(w, http.StatusConflict, "account already exists")
			return
		}
		writeProblem(w, http.StatusBadRequest, "invalid registration")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"userId": userID})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	pair, err := h.service.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	pair, _, err := h.service.Refresh(request.RefreshToken)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "missing bearer token")
		return
	}
	userID, err := h.service.AuthenticateAccess(token)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	h.service.Logout(userID)
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeProblem(w, http.StatusUnsupportedMediaType, "content type must be application/json")
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeProblem(w, http.StatusBadRequest, "request must contain one JSON object")
		return false
	}
	return true
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "about:blank",
		"title":  http.StatusText(status),
		"status": status,
		"detail": detail,
	})
}
