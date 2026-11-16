package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/domain/repository"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
)

type mockUserRepository struct {
	repository.UserRepository

	user   *models.User
	err    error
	wantID int64
	called bool
}

func (m *mockUserRepository) GetByID(
	ctx context.Context,
	id int64,
) (*models.User, error) {
	m.called = true

	if id != m.wantID {
		return nil, errors.New("unexpected user ID")
	}

	return m.user, m.err
}

func TestAdminMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		userID     int64
		user       *models.User
		repoErr    error
		wantStatus int
		wantNext   bool
		wantRepo   bool
	}{
		{
			name:       "missing user in context",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:   "employee access denied",
			userID: 5,
			user: &models.User{
				ID:   5,
				Role: models.RoleEmployee,
			},
			wantStatus: http.StatusForbidden,
			wantRepo:   true,
		},
		{
			name:   "admin access allowed",
			userID: 5,
			user: &models.User{
				ID:   5,
				Role: models.RoleAdmin,
			},
			wantStatus: http.StatusOK,
			wantNext:   true,
			wantRepo:   true,
		},
		{
			name:       "user not found",
			userID:     5,
			repoErr:    myErrors.ErrNotFound,
			wantStatus: http.StatusUnauthorized,
			wantRepo:   true,
		},
		{
			name:       "database error",
			userID:     5,
			repoErr:    errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantRepo:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepository{
				user:   tt.user,
				err:    tt.repoErr,
				wantID: tt.userID,
			}

			logger := slog.New(
				slog.NewTextHandler(io.Discard, nil),
			)

			server := &Server{
				Users:  userRepo,
				Logger: logger,
			}

			nextCalled := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(
				http.MethodPost,
				"/rooms",
				nil,
			)

			if tt.userID > 0 {
				ctx := context.WithValue(
					req.Context(),
					userIDKey,
					tt.userID,
				)
				req = req.WithContext(ctx)
			}

			rec := httptest.NewRecorder()

			handler := server.AdminMiddleware(next)
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

			if userRepo.called != tt.wantRepo {
				t.Errorf(
					"GetByID called = %v, want %v",
					userRepo.called,
					tt.wantRepo,
				)
			}

			if tt.repoErr == myErrors.ErrNotFound {
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
