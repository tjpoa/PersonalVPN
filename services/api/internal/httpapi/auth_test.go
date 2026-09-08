package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"personalvpn/services/api/internal/auth"
)

type fakeAuthService struct {
	registeredEmail string
	accessToken     string
	userID          string
	logoutUser      string
}

func (f *fakeAuthService) Register(_ context.Context, email, _ string) (string, error) {
	f.registeredEmail = email
	return "user-1", nil
}

func (f *fakeAuthService) Login(_ context.Context, _, _ string) (auth.TokenPair, error) {
	return auth.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func (f *fakeAuthService) Refresh(token string) (auth.TokenPair, string, error) {
	if token != "refresh" {
		return auth.TokenPair{}, "", errors.New("invalid")
	}
	return auth.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}, "user-1", nil
}

func (f *fakeAuthService) Logout(userID string) { f.logoutUser = userID }

func (f *fakeAuthService) AuthenticateAccess(token string) (string, error) {
	if token != f.accessToken {
		return "", errors.New("invalid")
	}
	return f.userID, nil
}

func TestRegisterRejectsUnknownFields(t *testing.T) {
	service := &fakeAuthService{}
	handler := NewAuthHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"u@example.com","password":"long-enough-password","unexpected":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Register(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("Register() status = %d, want 400", response.Code)
	}
}

func TestRefreshRotatesOnlyValidToken(t *testing.T) {
	service := &fakeAuthService{}
	handler := NewAuthHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"refreshToken":"refresh"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Refresh(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Refresh() status = %d, want 200", response.Code)
	}
	var pair auth.TokenPair
	if err := json.NewDecoder(response.Body).Decode(&pair); err != nil || pair.AccessToken != "new-access" {
		t.Fatalf("Refresh() body = %+v, error = %v", pair, err)
	}
}

func TestLogoutRequiresValidBearerAndRevokesUser(t *testing.T) {
	service := &fakeAuthService{accessToken: "access", userID: "user-1"}
	handler := NewAuthHandler(service)
	missing := httptest.NewRequest(http.MethodPost, "/logout", nil)
	missingResponse := httptest.NewRecorder()
	handler.Logout(missingResponse, missing)
	if missingResponse.Code != http.StatusUnauthorized {
		t.Fatalf("Logout(missing) status = %d, want 401", missingResponse.Code)
	}
	valid := httptest.NewRequest(http.MethodPost, "/logout", nil)
	valid.Header.Set("Authorization", "Bearer access")
	validResponse := httptest.NewRecorder()
	handler.Logout(validResponse, valid)
	if validResponse.Code != http.StatusNoContent || service.logoutUser != "user-1" {
		t.Fatalf("Logout(valid) = status %d user %q", validResponse.Code, service.logoutUser)
	}
}
