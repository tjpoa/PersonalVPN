package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	accessTokenLifetime  = 15 * time.Minute
	refreshTokenLifetime = 30 * 24 * time.Hour
	tokenPrefix          = "pv1_"
)

type TokenPair struct {
	AccessToken      string    `json:"accessToken"`
	RefreshToken     string    `json:"refreshToken"`
	AccessExpiresAt  time.Time `json:"accessExpiresAt"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
}

type session struct {
	userID    string
	familyID  string
	expiresAt time.Time
	kind      string
}

type SessionStore struct {
	mu       sync.Mutex
	sessions map[[32]byte]session
	families map[string]map[[32]byte]struct{}
	consumed map[[32]byte]string
	now      func() time.Time
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[[32]byte]session),
		families: make(map[string]map[[32]byte]struct{}),
		consumed: make(map[[32]byte]string),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (s *SessionStore) Issue(userID string) (TokenPair, error) {
	if userID == "" {
		return TokenPair{}, errors.New("user id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	familyID, err := randomToken()
	if err != nil {
		return TokenPair{}, err
	}
	family := make(map[[32]byte]struct{})
	s.families[familyID] = family
	access, err := s.issueLocked(userID, familyID, "access", now.Add(accessTokenLifetime))
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := s.issueLocked(userID, familyID, "refresh", now.Add(refreshTokenLifetime))
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access.token, RefreshToken: refresh.token, AccessExpiresAt: access.expiresAt, RefreshExpiresAt: refresh.expiresAt}, nil
}

func (s *SessionStore) AuthenticateAccess(token string) (string, bool) {
	return s.authenticate(token, "access")
}

// RotateRefresh atomically invalidates a refresh token and issues a new pair.
// Reuse of an already-consumed token revokes the whole token family.
func (s *SessionStore) RotateRefresh(token string) (TokenPair, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, ok := tokenHash(token)
	if !ok {
		return TokenPair{}, "", false
	}
	old, exists := s.sessions[hash]
	if !exists || old.kind != "refresh" || !s.now().Before(old.expiresAt) {
		if exists {
			s.revokeFamilyLocked(old.familyID)
		} else if consumedUserID, consumed := s.consumed[hash]; consumed {
			s.revokeUserLocked(consumedUserID)
		}
		return TokenPair{}, "", false
	}
	s.consumed[hash] = old.userID
	s.revokeFamilyLocked(old.familyID)
	// Keep one new family: the old family is deliberately unusable after rotation.
	familyID, err := randomToken()
	if err != nil {
		return TokenPair{}, "", false
	}
	s.families[familyID] = make(map[[32]byte]struct{})
	now := s.now()
	access, err := s.issueLocked(old.userID, familyID, "access", now.Add(accessTokenLifetime))
	if err != nil {
		return TokenPair{}, "", false
	}
	refresh, err := s.issueLocked(old.userID, familyID, "refresh", now.Add(refreshTokenLifetime))
	if err != nil {
		return TokenPair{}, "", false
	}
	return TokenPair{AccessToken: access.token, RefreshToken: refresh.token, AccessExpiresAt: access.expiresAt, RefreshExpiresAt: refresh.expiresAt}, old.userID, true
}

func (s *SessionStore) RevokeUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for familyID, hashes := range s.families {
		for hash := range hashes {
			if record, exists := s.sessions[hash]; exists && record.userID == userID {
				s.revokeFamilyLocked(familyID)
				break
			}
		}
	}
}

type issuedToken struct {
	token     string
	expiresAt time.Time
}

func (s *SessionStore) issueLocked(userID, familyID, kind string, expiresAt time.Time) (issuedToken, error) {
	token, err := randomToken()
	if err != nil {
		return issuedToken{}, err
	}
	hash, _ := tokenHash(token)
	s.sessions[hash] = session{userID: userID, familyID: familyID, expiresAt: expiresAt, kind: kind}
	s.families[familyID][hash] = struct{}{}
	return issuedToken{token: token, expiresAt: expiresAt}, nil
}

func (s *SessionStore) authenticate(token, kind string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, ok := tokenHash(token)
	if !ok {
		return "", false
	}
	record, exists := s.sessions[hash]
	if !exists || record.kind != kind || !s.now().Before(record.expiresAt) {
		return "", false
	}
	return record.userID, true
}

func (s *SessionStore) revokeFamilyLocked(familyID string) {
	for hash := range s.families[familyID] {
		delete(s.sessions, hash)
	}
	delete(s.families, familyID)
}

func (s *SessionStore) revokeUserLocked(userID string) {
	for familyID, hashes := range s.families {
		for hash := range hashes {
			if record, exists := s.sessions[hash]; exists && record.userID == userID {
				s.revokeFamilyLocked(familyID)
				break
			}
		}
	}
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(value), nil
}

func tokenHash(token string) ([32]byte, bool) {
	var zero [32]byte
	if len(token) != len(tokenPrefix)+43 || len(token) < len(tokenPrefix) || token[:len(tokenPrefix)] != tokenPrefix {
		return zero, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token[len(tokenPrefix):])
	if err != nil || len(decoded) != 32 {
		return zero, false
	}
	return sha256.Sum256([]byte(token)), true
}
