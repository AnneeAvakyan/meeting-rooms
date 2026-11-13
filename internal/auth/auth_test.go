package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newTestTokenManager(t *testing.T) *TokenManager {
	t.Helper()

	tm, err := NewTokenManager(
		[]byte("0123456789abcdef0123456789abcdef"),
		time.Hour,
	)
	if err != nil {
		t.Fatalf("NewTokenManager failed: %v", err)
	}

	return tm
}

func TestTokenManager_GenerateAndValidate(t *testing.T) {
	tm := newTestTokenManager(t)

	token, err := tm.GenerateToken(42)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	userID, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if userID != 42 {
		t.Errorf("expected user ID 42, got %d", userID)
	}
}

func TestTokenManager_InvalidToken(t *testing.T) {
	tm := newTestTokenManager(t)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "malformed token",
			token: "not.a.valid.jwt",
		},
		{
			name:  "random string",
			token: "invalid-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tm.ValidateToken(tt.token)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestTokenManager_ExpiredToken(t *testing.T) {
	tm := newTestTokenManager(t)

	claims := jwt.RegisteredClaims{
		Subject:   "42",
		Issuer:    "meeting-rooms",
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(tm.secretKey)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	_, err = tm.ValidateToken(signed)
	if err == nil {
		t.Error("expected expired token to be rejected")
	}
}

func TestTokenManager_WrongSecret(t *testing.T) {
	tm := newTestTokenManager(t)

	token, err := tm.GenerateToken(42)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	other, err := NewTokenManager(
		[]byte("abcdef0123456789abcdef0123456789"),
		time.Hour,
	)
	if err != nil {
		t.Fatalf("NewTokenManager failed: %v", err)
	}

	_, err = other.ValidateToken(token)
	if err == nil {
		t.Error("expected token signed with another key to be rejected")
	}
}

func TestTokenManager_ModifiedPayload(t *testing.T) {
	tm := newTestTokenManager(t)

	token, err := tm.GenerateToken(42)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	// Подменяем payload, оставляя старую подпись.
	parts[1] = base64.RawURLEncoding.EncodeToString(
		[]byte(`{"sub":"1","iss":"meeting-rooms"}`),
	)
	modified := strings.Join(parts, ".")

	_, err = tm.ValidateToken(modified)
	if err == nil {
		t.Error("expected modified JWT to be rejected")
	}
}

func TestTokenManager_InvalidSubject(t *testing.T) {
	tm := newTestTokenManager(t)

	for _, subject := range []string{"", "abc", "0", "-5"} {
		t.Run("subject_"+subject, func(t *testing.T) {
			now := time.Now()

			claims := jwt.RegisteredClaims{
				Subject:   subject,
				Issuer:    "meeting-rooms",
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signed, err := token.SignedString(tm.secretKey)
			if err != nil {
				t.Fatalf("failed to sign test token: %v", err)
			}

			_, err = tm.ValidateToken(signed)
			if err == nil {
				t.Errorf("expected subject %q to be rejected", subject)
			}
		})
	}
}
