package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/domain/repository"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
	"golang.org/x/crypto/bcrypt"
)

type mockUserRepo struct {
	repository.UserRepository

	createdUser *models.User
	createErr   error
	createCalls int

	foundUser      *models.User
	getErr         error
	requestedEmail string
	getCalls       int
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	m.createCalls++
	m.createdUser = user

	if m.createErr != nil {
		return m.createErr
	}

	user.ID = 42
	return nil
}

func (m *mockUserRepo) GetByEmail(
	ctx context.Context,
	email string,
) (*models.User, error) {
	m.getCalls++
	m.requestedEmail = email

	if m.getErr != nil {
		return nil, m.getErr
	}

	return m.foundUser, nil
}

type mockTokenGenerator struct {
	token     string
	err       error
	userID    int64
	callCount int
}

func (m *mockTokenGenerator) GenerateToken(userID int64) (string, error) {
	m.callCount++
	m.userID = userID

	if m.err != nil {
		return "", m.err
	}

	return m.token, nil
}

func TestUserServiceRegister(t *testing.T) {
	tests := []struct {
		name        string
		input       RegisterInput
		createErr   error
		wantErr     error
		wantCreated bool
	}{
		{
			name: "successful registration",
			input: RegisterInput{
				Name:     "  Anna  ",
				Email:    "  ANNA@EXAMPLE.COM  ",
				Password: "securePassword123",
			},
			wantCreated: true,
		},
		{
			name: "empty name",
			input: RegisterInput{
				Name:     " ",
				Email:    "anna@example.com",
				Password: "securePassword123",
			},
			wantErr: myErrors.ErrInvalidInput,
		},
		{
			name: "empty email",
			input: RegisterInput{
				Name:     "Anna",
				Email:    " ",
				Password: "securePassword123",
			},
			wantErr: myErrors.ErrInvalidInput,
		},
		{
			name: "short password",
			input: RegisterInput{
				Name:     "Anna",
				Email:    "anna@example.com",
				Password: "1234567",
			},
			wantErr: myErrors.ErrInvalidInput,
		},
		{
			name: "password longer than 72 bytes",
			input: RegisterInput{
				Name:     "Anna",
				Email:    "anna@example.com",
				Password: strings.Repeat("a", 73),
			},
			wantErr: myErrors.ErrInvalidInput,
		},
		{
			name: "duplicate email",
			input: RegisterInput{
				Name:     "Anna",
				Email:    "anna@example.com",
				Password: "securePassword123",
			},
			createErr: myErrors.ErrEmailExists,
			wantErr:   myErrors.ErrEmailExists,
		},
		{
			name: "repository error",
			input: RegisterInput{
				Name:     "Anna",
				Email:    "anna@example.com",
				Password: "securePassword123",
			},
			createErr: errors.New("database unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockUserRepo{
				createErr: tt.createErr,
			}
			service := NewUserService(repo, &mockTokenGenerator{})

			user, err := service.Register(
				context.Background(),
				tt.input,
			)

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Register() error = %v, want %v", err, tt.wantErr)
				}
			case tt.createErr != nil:
				if !errors.Is(err, tt.createErr) {
					t.Fatalf("Register() error = %v, want %v", err, tt.createErr)
				}
			default:
				if err != nil {
					t.Fatalf("Register() unexpected error: %v", err)
				}
			}

			if tt.wantErr != nil || tt.createErr != nil {
				if user != nil {
					t.Errorf("Register() user = %v, want nil", user)
				}
			}

			if tt.wantErr == myErrors.ErrInvalidInput {
				if repo.createCalls != 0 {
					t.Errorf("Create called %d times, want 0", repo.createCalls)
				}
			}

			if !tt.wantCreated {
				return
			}

			if user == nil {
				t.Fatal("Register() returned nil user")
			}

			if repo.createCalls != 1 {
				t.Errorf("Create calls = %d, want 1", repo.createCalls)
			}

			if user.ID != 42 {
				t.Errorf("user ID = %d, want 42", user.ID)
			}

			if user.Name != "Anna" {
				t.Errorf("name = %q, want Anna", user.Name)
			}

			if user.Email != "anna@example.com" {
				t.Errorf("email = %q, want anna@example.com", user.Email)
			}

			if user.Role != models.RoleEmployee {
				t.Errorf("role = %q, want employee", user.Role)
			}

			if user.PasswordHash == tt.input.Password {
				t.Error("password stored without hashing")
			}

			if err := bcrypt.CompareHashAndPassword(
				[]byte(user.PasswordHash),
				[]byte(tt.input.Password),
			); err != nil {
				t.Errorf("password hash verification failed: %v", err)
			}
		})
	}
}

func TestUserServiceLogin(t *testing.T) {
	password := "securePassword123"

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("failed to generate test password hash: %v", err)
	}

	validUser := &models.User{
		ID:           42,
		Email:        "anna@example.com",
		PasswordHash: string(hash),
		Role:         models.RoleEmployee,
	}

	tests := []struct {
		name           string
		input          LoginInput
		foundUser      *models.User
		getErr         error
		tokenErr       error
		wantErr        error
		wantSuccess    bool
		wantTokenCalls int
	}{
		{
			name: "successful login",
			input: LoginInput{
				Email:    " ANNA@EXAMPLE.COM ",
				Password: password,
			},
			foundUser:      validUser,
			wantSuccess:    true,
			wantTokenCalls: 1,
		},
		{
			name: "user not found",
			input: LoginInput{
				Email:    "missing@example.com",
				Password: password,
			},
			getErr:  myErrors.ErrNotFound,
			wantErr: myErrors.ErrInvalidCredentials,
		},
		{
			name: "wrong password",
			input: LoginInput{
				Email:    "anna@example.com",
				Password: "wrongPassword",
			},
			foundUser: validUser,
			wantErr:   myErrors.ErrInvalidCredentials,
		},
		{
			name: "repository error",
			input: LoginInput{
				Email:    "anna@example.com",
				Password: password,
			},
			getErr: errors.New("database unavailable"),
		},
		{
			name: "token generation error",
			input: LoginInput{
				Email:    "anna@example.com",
				Password: password,
			},
			foundUser:      validUser,
			tokenErr:       errors.New("cannot generate token"),
			wantTokenCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockUserRepo{
				foundUser: tt.foundUser,
				getErr:    tt.getErr,
			}

			generator := &mockTokenGenerator{
				token: "test-jwt-token",
				err:   tt.tokenErr,
			}

			service := NewUserService(repo, generator)

			result, err := service.Login(
				context.Background(),
				tt.input,
			)

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Login() error = %v, want %v", err, tt.wantErr)
				}
			case tt.getErr != nil:
				if !errors.Is(err, tt.getErr) {
					t.Fatalf("Login() error = %v, want %v", err, tt.getErr)
				}
			case tt.tokenErr != nil:
				if !errors.Is(err, tt.tokenErr) {
					t.Fatalf("Login() error = %v, want %v", err, tt.tokenErr)
				}
			default:
				if err != nil {
					t.Fatalf("Login() unexpected error: %v", err)
				}
			}

			if repo.getCalls != 1 {
				t.Errorf("GetByEmail calls = %d, want 1", repo.getCalls)
			}

			if generator.callCount != tt.wantTokenCalls {
				t.Errorf(
					"GenerateToken calls = %d, want %d",
					generator.callCount,
					tt.wantTokenCalls,
				)
			}

			if !tt.wantSuccess {
				if result != nil {
					t.Errorf("Login() result = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Login() returned nil result")
			}

			if repo.requestedEmail != "anna@example.com" {
				t.Errorf(
					"requested email = %q, want anna@example.com",
					repo.requestedEmail,
				)
			}

			if generator.userID != 42 {
				t.Errorf("token userID = %d, want 42", generator.userID)
			}

			if result.AccessToken != "test-jwt-token" {
				t.Errorf("access token = %q", result.AccessToken)
			}

			if result.TokenType != "Bearer" {
				t.Errorf("token type = %q, want Bearer", result.TokenType)
			}
		})
	}
}
