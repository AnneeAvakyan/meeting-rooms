package auth

import (
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	secretKey  []byte
	timeToLive time.Duration
}

func NewTokenManager(key []byte, timeToLive time.Duration) (*TokenManager, error) {
	if utf8.RuneCountInString(string(key)) < 32 {
		return nil, fmt.Errorf("key must be 32 runes, got %d", len(key))
	}
	if timeToLive < 0 {
		return nil, fmt.Errorf("timeToLive must be greater or equal to 0, got %d", timeToLive)
	}

	return &TokenManager{
		secretKey:  key,
		timeToLive: timeToLive,
	}, nil
}

func (tm *TokenManager) GenerateToken(userID int64) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		Issuer:    "meeting-rooms",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tm.timeToLive)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(tm.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return signedToken, nil
}

func (tm *TokenManager) ValidateToken(tokenString string) (int64, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (interface{}, error) {
			return tm.secretKey, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer("meeting-rooms"),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to parse token: %w", err)
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse user id: %w", err)
	}

	if userID <= 0 {
		return 0, fmt.Errorf("invalid user id: %d", userID)
	}

	return userID, nil
}
