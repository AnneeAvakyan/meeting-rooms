package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/domain/repository"
	"github.com/AnneeAvakyan/meeting-rooms/internal/services"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
	"golang.org/x/crypto/bcrypt"
)

type authTestUserRepo struct {
	repository.UserRepository

	user      *models.User
	createErr error
	getErr    error
}

func (r *authTestUserRepo) Create(
	ctx context.Context, user *models.User,
) error {
	if r.createErr != nil {
		return r.createErr
	}

	user.ID = 42
	return nil
}

func (r *authTestUserRepo) GetByEmail(
	ctx context.Context, email string,
) (*models.User, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.user, nil
}

type authTestTokenGenerator struct{}

func (authTestTokenGenerator) GenerateToken(
	userID int64,
) (string, error) {
	return "test-access-token", nil
}

func newTestAuthHandlers(repo *authTestUserRepo) *AuthHandlers {
	service := services.NewUserService(
		repo,
		authTestTokenGenerator{},
	)

	return &AuthHandlers{
		userService: service,
	}
}

func TestRegisterHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
	}{
		{
			name: "successful registration",
			body: `{
				"name": "Anna",
				"email": "anna@example.com",
				"password": "strongPassword123"
			}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid JSON",
			body:       `{"name":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty name",
			body: `{
				"name": "",
				"email": "anna@example.com",
				"password": "strongPassword123"
			}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "short password",
			body: `{
				"name": "Anna",
				"email": "anna@example.com",
				"password": "123"
			}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate email",
			body: `{
				"name": "Anna",
				"email": "anna@example.com",
				"password": "strongPassword123"
			}`,
			createErr:  myErrors.ErrEmailExists,
			wantStatus: http.StatusConflict,
		},
		{
			name: "database error",
			body: `{
				"name": "Anna",
				"email": "anna@example.com",
				"password": "strongPassword123"
			}`,
			createErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &authTestUserRepo{
				createErr: tt.createErr,
			}
			handler := newTestAuthHandlers(repo)

			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/register",
				strings.NewReader(tt.body),
			)
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if rec.Code == http.StatusCreated {
				var response RegisterResponse
				if err := json.Unmarshal(
					rec.Body.Bytes(), &response,
				); err != nil {
					t.Fatal(err)
				}

				if response.ID != 42 {
					t.Errorf("ID = %d, want 42", response.ID)
				}
				if response.Role != models.RoleEmployee {
					t.Errorf(
						"role = %q, want employee",
						response.Role,
					)
				}

				var raw map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
					t.Fatal(err)
				}

				if _, ok := raw["password_hash"]; ok {
					t.Error("response exposes password_hash")
				}
			}
		})
	}
}

func TestLoginHandler(t *testing.T) {
	password := "strongPassword123"

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	user := &models.User{
		ID:           42,
		Email:        "anna@example.com",
		PasswordHash: string(hash),
		Role:         models.RoleEmployee,
	}

	tests := []struct {
		name       string
		body       string
		getErr     error
		wantStatus int
	}{
		{
			name: "successful login",
			body: `{
				"email": "anna@example.com",
				"password": "strongPassword123"
			}`,
			wantStatus: http.StatusOK,
		},
		{
			name: "wrong password",
			body: `{
				"email": "anna@example.com",
				"password": "incorrect-password"
			}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid JSON",
			body:       `{"email":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "user not found",
			body: `{
				"email": "missing@example.com",
				"password": "strongPassword123"
			}`,
			getErr:     myErrors.ErrNotFound,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "database error",
			body: `{
				"email": "anna@example.com",
				"password": "strongPassword123"
			}`,
			getErr:     errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &authTestUserRepo{
				user:   user,
				getErr: tt.getErr,
			}
			handler := newTestAuthHandlers(repo)

			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/login",
				strings.NewReader(tt.body),
			)
			rec := httptest.NewRecorder()

			handler.Login(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if rec.Code == http.StatusOK {
				var response LoginResponse

				if err := json.Unmarshal(
					rec.Body.Bytes(), &response,
				); err != nil {
					t.Fatal(err)
				}

				if response.AccessToken != "test-access-token" {
					t.Errorf(
						"token = %q, want test-access-token",
						response.AccessToken,
					)
				}
				if response.TokenType != "Bearer" {
					t.Errorf(
						"token type = %q, want Bearer",
						response.TokenType,
					)
				}
			}
		})
	}
}
