package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/internal/services"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
)

type AuthHandlers struct {
	userService *services.UserService
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"token"`
	TokenType   string `json:"token_type"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type RegisterResponse struct {
	ID    int64       `json:"id"`
	Name  string      `json:"name"`
	Email string      `json:"email"`
	Role  models.Role `json:"role"`
}

func NewAuthHandlers(userService *services.UserService) *AuthHandlers {
	return &AuthHandlers{userService}
}

func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var input LoginRequest
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loginInput := services.LoginInput{
		Password: input.Password,
		Email:    input.Email,
	}

	login, err := h.userService.Login(r.Context(), loginInput)
	if err != nil {
		if errors.Is(err, myErrors.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, myErrors.ErrInvalidCredentials)
			return
		}
		slog.Error("failed to login", "error", err)
		writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
		return
	}

	loginResponse := LoginResponse{
		AccessToken: login.AccessToken,
		TokenType:   login.TokenType,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(loginResponse)
}

func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var input RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	registerInput := services.RegisterInput{
		Email:    input.Email,
		Password: input.Password,
		Name:     input.Name,
	}
	registeredUser, err := h.userService.Register(r.Context(), registerInput)
	if err != nil {
		if errors.Is(err, myErrors.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, myErrors.ErrInvalidInput)
			return
		}
		if errors.Is(err, myErrors.ErrEmailExists) {
			writeError(w, http.StatusConflict, myErrors.ErrEmailExists)
			return
		}
		slog.Error("registration failed", "error", err)
		writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
		return
	}

	result := RegisterResponse{
		ID:    registeredUser.ID,
		Name:  registeredUser.Name,
		Email: registeredUser.Email,
		Role:  registeredUser.Role,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}
