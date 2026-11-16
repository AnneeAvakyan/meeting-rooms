package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockTokenValidator struct {
	userID    int64
	err       error
	wantToken string
	called    bool
}

func (m *mockTokenValidator) ValidateToken(token string) (int64, error) {
	m.called = true

	if m.wantToken != "" && token != m.wantToken {
		return 0, errors.New("unexpected token")
	}

	return m.userID, m.err
}

func TestUserIDFromContext(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		wantID int64
		wantOK bool
	}{
		{
			name:   "valid user ID",
			value:  int64(5),
			wantID: 5,
			wantOK: true,
		},
		{
			name:   "zero user ID",
			value:  int64(0),
			wantID: 0,
			wantOK: false,
		},
		{
			name:   "negative user ID",
			value:  int64(-1),
			wantID: 0,
			wantOK: false,
		},
		{
			name:   "wrong value type",
			value:  "5",
			wantID: 0,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(
				context.Background(),
				userIDKey,
				tt.value,
			)

			gotID, gotOK := UserIDFromContext(ctx)

			if gotID != tt.wantID || gotOK != tt.wantOK {
				t.Errorf(
					"UserIDFromContext() = (%d, %v), want (%d, %v)",
					gotID,
					gotOK,
					tt.wantID,
					tt.wantOK,
				)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		userID        int64
		validatorErr  error
		wantStatus    int
		wantNext      bool
		wantValidate  bool
	}{
		{
			name:       "missing authorization header",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "basic authorization scheme",
			authorization: "Basic abc123",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "missing token",
			authorization: "Bearer",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "too many authorization parts",
			authorization: "Bearer token extra",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "invalid token",
			authorization: "Bearer invalid-token",
			validatorErr:  errors.New("invalid signature"),
			wantStatus:    http.StatusUnauthorized,
			wantValidate:  true,
		},
		{
			name:          "expired token",
			authorization: "Bearer expired-token",
			validatorErr:  errors.New("token expired"),
			wantStatus:    http.StatusUnauthorized,
			wantValidate:  true,
		},
		{
			name:          "valid token",
			authorization: "Bearer valid-token",
			userID:        5,
			wantStatus:    http.StatusOK,
			wantNext:      true,
			wantValidate:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &mockTokenValidator{
				userID:    tt.userID,
				err:       tt.validatorErr,
				wantToken: "valid-token",
			}

			handlers := &AuthHandlers{
				tokenValidator: validator,
			}

			nextCalled := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				userID, ok := UserIDFromContext(r.Context())
				if !ok {
					t.Error("user ID missing from context")
				}

				if userID != tt.userID {
					t.Errorf(
						"context userID = %d, want %d",
						userID,
						tt.userID,
					)
				}

				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(
				http.MethodGet,
				"/rooms",
				nil,
			)

			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			rec := httptest.NewRecorder()

			handler := handlers.AuthMiddleware(next)
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d",
					rec.Code,
					tt.wantStatus,
				)
			}

			if nextCalled != tt.wantNext {
				t.Errorf(
					"nextCalled = %v, want %v",
					nextCalled,
					tt.wantNext,
				)
			}

			if validator.called != tt.wantValidate {
				t.Errorf(
					"validator called = %v, want %v",
					validator.called,
					tt.wantValidate,
				)
			}

			if tt.wantStatus == http.StatusUnauthorized {
				if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
					t.Errorf(
						"WWW-Authenticate = %q, want Bearer",
						got,
					)
				}
			}
		})
	}
}
