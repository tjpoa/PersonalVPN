package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("HashPassword() = %q, want Argon2id PHC", hash)
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Fatal("VerifyPassword() rejected correct password")
	}
	if VerifyPassword("incorrect horse battery staple", hash) {
		t.Fatal("VerifyPassword() accepted incorrect password")
	}
	if VerifyPassword("correct horse battery staple", strings.Replace(hash, "m=65536", "m=1", 1)) {
		t.Fatal("VerifyPassword() accepted unsafe memory parameter")
	}
}

func TestPasswordLengthLimits(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("HashPassword() accepted short password")
	}
	if _, err := HashPassword(strings.Repeat("x", 1025)); err == nil {
		t.Fatal("HashPassword() accepted oversized password")
	}
}

func TestSessionRotationAndReuseRevocation(t *testing.T) {
	store := NewSessionStore()
	store.now = func() time.Time { return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC) }
	pair, err := store.Issue("user-a")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	for name, token := range map[string]string{"access": pair.AccessToken, "refresh": pair.RefreshToken} {
		if !strings.HasPrefix(token, "pv1_") || len(token) != 47 {
			t.Fatalf("%s token has length/prefix (%d, %q), want pv1_ plus 43 characters", name, len(token), token[:min(len(token), 8)])
		}
	}
	if user, ok := store.AuthenticateAccess(pair.AccessToken); !ok || user != "user-a" {
		t.Fatalf("AuthenticateAccess() = (%q, %v)", user, ok)
	}
	rotated, user, ok := store.RotateRefresh(pair.RefreshToken)
	if !ok || user != "user-a" || rotated.RefreshToken == pair.RefreshToken {
		t.Fatalf("RotateRefresh() = (%+v, %q, %v)", rotated, user, ok)
	}
	if _, _, ok := store.RotateRefresh(pair.RefreshToken); ok {
		t.Fatal("RotateRefresh() accepted refresh-token reuse")
	}
	if _, ok := store.AuthenticateAccess(rotated.AccessToken); ok {
		t.Fatal("reuse detection did not revoke rotated token family")
	}
}

func TestExpiredAccessTokenRejected(t *testing.T) {
	store := NewSessionStore()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	pair, err := store.Issue("user-a")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	now = now.Add(16 * time.Minute)
	if _, ok := store.AuthenticateAccess(pair.AccessToken); ok {
		t.Fatal("AuthenticateAccess() accepted expired token")
	}
}
