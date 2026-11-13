package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/domain/repository"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
	"golang.org/x/crypto/bcrypt"
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	AccessToken string
	TokenType   string
}

type UserService struct {
	userRepo       repository.UserRepository
	tokenGenerator TokenGenerator
}

func NewUserService(userRepo repository.UserRepository, generator TokenGenerator) *UserService {
	return &UserService{
		userRepo:       userRepo,
		tokenGenerator: generator,
	}
}

func (s *UserService) Register(ctx context.Context, input RegisterInput) (*models.User, error) {
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if name == "" || email == "" {
		return nil, myErrors.ErrInvalidInput
	}

	lenPassword := utf8.RuneCountInString(input.Password)
	if lenPassword < 8 {
		return nil, fmt.Errorf("%w: password is too short", myErrors.ErrInvalidInput)
	}
	if len([]byte(input.Password)) > 72 {
		return nil, fmt.Errorf("%w: password is too long", myErrors.ErrInvalidInput)
	}

	password := []byte(input.Password)
	hash, err := bcrypt.GenerateFromPassword(
		password,
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	passwordHash := string(hash)
	user := &models.User{
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         models.RoleEmployee,
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (s *UserService) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, myErrors.ErrNotFound) {
			return nil, myErrors.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, myErrors.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to compare password: %w", err)
	}

	token, err := s.tokenGenerator.GenerateToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	res := &LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
	}

	return res, nil
}
