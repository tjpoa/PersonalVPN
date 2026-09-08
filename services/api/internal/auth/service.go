package auth

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type UserStore interface {
	CreateUser(ctx context.Context, normalizedEmail, passwordHash string) (string, error)
	FindUserPasswordHash(ctx context.Context, normalizedEmail string) (userID, passwordHash string, disabled bool, err error)
}

type Service struct {
	users    UserStore
	sessions *SessionStore
}

func NewService(users UserStore, sessions *SessionStore) *Service {
	if sessions == nil {
		sessions = NewSessionStore()
	}
	return &Service{users: users, sessions: sessions}
}

func (s *Service) Register(ctx context.Context, email, password string) (string, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return "", err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return "", err
	}
	return s.users.CreateUser(ctx, normalizedEmail, hash)
}

func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}
	userID, passwordHash, disabled, err := s.users.FindUserPasswordHash(ctx, normalizedEmail)
	if err != nil || disabled || !VerifyPassword(password, passwordHash) {
		return TokenPair{}, ErrInvalidCredentials
	}
	return s.sessions.Issue(userID)
}

func (s *Service) Refresh(refreshToken string) (TokenPair, string, error) {
	pair, userID, ok := s.sessions.RotateRefresh(refreshToken)
	if !ok {
		return TokenPair{}, "", ErrInvalidCredentials
	}
	return pair, userID, nil
}

func (s *Service) Logout(userID string) {
	s.sessions.RevokeUser(userID)
}

func (s *Service) AuthenticateAccess(accessToken string) (string, error) {
	userID, ok := s.sessions.AuthenticateAccess(accessToken)
	if !ok {
		return "", ErrInvalidCredentials
	}
	return userID, nil
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if len(normalized) < 3 || len(normalized) > 320 || strings.ContainsAny(normalized, "\r\n") || !strings.Contains(normalized, "@") {
		return "", ErrInvalidCredentials
	}
	return normalized, nil
}
