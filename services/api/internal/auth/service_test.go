package auth

import (
	"context"
	"errors"
	"testing"
)

type fakeUserStore struct {
	userID       string
	passwordHash string
	disabled     bool
	email        string
}

func (f *fakeUserStore) CreateUser(_ context.Context, email, passwordHash string) (string, error) {
	f.email, f.passwordHash = email, passwordHash
	if f.userID == "" {
		f.userID = "user-1"
	}
	return f.userID, nil
}

func (f *fakeUserStore) FindUserPasswordHash(_ context.Context, email string) (string, string, bool, error) {
	if email != f.email {
		return "", "", false, errors.New("not found")
	}
	return f.userID, f.passwordHash, f.disabled, nil
}

func TestServiceRegisterNormalizesEmailAndHashesPassword(t *testing.T) {
	users := &fakeUserStore{}
	service := NewService(users, nil)
	userID, err := service.Register(context.Background(), "  User@Example.COM ", "correct horse battery staple")
	if err != nil || userID != "user-1" {
		t.Fatalf("Register() = (%q, %v)", userID, err)
	}
	if users.email != "user@example.com" || !VerifyPassword("correct horse battery staple", users.passwordHash) {
		t.Fatalf("Register() stored email/hash incorrectly: email=%q hash=%q", users.email, users.passwordHash)
	}
}

func TestServiceLoginRefreshAndLogout(t *testing.T) {
	users := &fakeUserStore{}
	service := NewService(users, nil)
	if _, err := service.Register(context.Background(), "user@example.com", "correct horse battery staple"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	pair, err := service.Login(context.Background(), "USER@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	userID, err := service.AuthenticateAccess(pair.AccessToken)
	if err != nil || userID != "user-1" {
		t.Fatalf("AuthenticateAccess() = (%q, %v)", userID, err)
	}
	rotated, userID, err := service.Refresh(pair.RefreshToken)
	if err != nil || userID != "user-1" {
		t.Fatalf("Refresh() = (%+v, %q, %v)", rotated, userID, err)
	}
	service.Logout("user-1")
	if _, err := service.AuthenticateAccess(rotated.AccessToken); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("AuthenticateAccess() after logout = %v", err)
	}
}

func TestServiceRejectsDisabledAndWrongUsers(t *testing.T) {
	users := &fakeUserStore{}
	service := NewService(users, nil)
	if _, err := service.Register(context.Background(), "user@example.com", "correct horse battery staple"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	users.disabled = true
	if _, err := service.Login(context.Background(), "user@example.com", "correct horse battery staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login(disabled) error = %v", err)
	}
	users.disabled = false
	if _, err := service.Login(context.Background(), "other@example.com", "correct horse battery staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login(other) error = %v", err)
	}
}
