package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/domain/repository"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
)

type mockBookingRepository struct {
	repository.BookingRepository

	booking   *models.Booking
	bookings  []models.Booking
	getErr    error
	listErr   error
	createErr error
	updateErr error
	cancelErr error

	createCalled bool
	updateCalled bool
	cancelCalled bool
	cancelUserID int64
	cancelID     int64
}

func (m *mockBookingRepository) GetByID(
	ctx context.Context, id int64,
) (*models.Booking, error) {
	return m.booking, m.getErr
}

func (m *mockBookingRepository) ListByRoom(
	ctx context.Context, roomID int64,
) ([]models.Booking, error) {
	return m.bookings, m.listErr
}

func (m *mockBookingRepository) Create(
	ctx context.Context, booking *models.Booking,
) error {
	m.createCalled = true

	if m.createErr != nil {
		return m.createErr
	}

	booking.ID = 10
	return nil
}

func (m *mockBookingRepository) Update(
	ctx context.Context, booking *models.Booking,
) error {
	m.updateCalled = true
	return m.updateErr
}

func (m *mockBookingRepository) Cancel(
	ctx context.Context, userID int64, id int64,
) error {
	m.cancelCalled = true
	m.cancelUserID = userID
	m.cancelID = id
	return m.cancelErr
}

const validBookingJSON = `{
	"room_id": 1,
	"start_time": "2026-11-18T10:00:00+03:00",
	"end_time": "2026-11-18T11:00:00+03:00"
}`

func bookingTestRequest(
	method string,
	path string,
	body string,
	userID int64,
) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if userID > 0 {
		ctx := context.WithValue(req.Context(), userIDKey, userID)
		req = req.WithContext(ctx)
	}

	return req
}

func bookingTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCreateBooking(t *testing.T) {
	conflictingBooking := models.Booking{
		ID:        2,
		RoomID:    1,
		UserID:    9,
		StartTime: bookingTime("2026-11-18T10:30:00+03:00"),
		EndTime:   bookingTime("2026-11-18T11:30:00+03:00"),
		Status:    models.StatusActive,
	}

	tests := []struct {
		name       string
		body       string
		userID     int64
		existing   []models.Booking
		createErr  error
		wantStatus int
		wantCreate bool
	}{
		{
			name:       "successful creation",
			body:       validBookingJSON,
			userID:     5,
			wantStatus: http.StatusCreated,
			wantCreate: true,
		},
		{
			name:       "unauthorized user",
			body:       validBookingJSON,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid JSON",
			body:       `{"room_id":`,
			userID:     5,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid time interval",
			body: `{
				"room_id": 1,
				"start_time": "2026-11-18T12:00:00+03:00",
				"end_time": "2026-11-18T11:00:00+03:00"
			}`,
			userID:     5,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "overlapping booking",
			body:       validBookingJSON,
			userID:     5,
			existing:   []models.Booking{conflictingBooking},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "repository detects overlap",
			body:       validBookingJSON,
			userID:     5,
			createErr:  myErrors.ErrOverlap,
			wantStatus: http.StatusConflict,
			wantCreate: true,
		},
		{
			name:       "repository error",
			body:       validBookingJSON,
			userID:     5,
			createErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockBookingRepository{
				bookings:  tt.existing,
				createErr: tt.createErr,
			}

			server := &Server{Bookings: repo}

			req := bookingTestRequest(
				http.MethodPost, "/bookings", tt.body, tt.userID,
			)
			rec := httptest.NewRecorder()

			server.createBooking(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body = %s",
					rec.Code, tt.wantStatus, rec.Body.String(),
				)
			}

			if repo.createCalled != tt.wantCreate {
				t.Errorf(
					"Create called = %v, want %v",
					repo.createCalled, tt.wantCreate,
				)
			}
		})
	}
}

func TestUpdateBooking(t *testing.T) {
	ownedBooking := &models.Booking{
		ID:     7,
		RoomID: 1,
		UserID: 5,
		Status: models.StatusActive,
	}

	otherBooking := &models.Booking{
		ID:     7,
		RoomID: 1,
		UserID: 9,
		Status: models.StatusActive,
	}

	conflict := models.Booking{
		ID:        8,
		RoomID:    1,
		UserID:    9,
		StartTime: bookingTime("2026-11-18T10:30:00+03:00"),
		EndTime:   bookingTime("2026-11-18T11:30:00+03:00"),
		Status:    models.StatusActive,
	}

	tests := []struct {
		name       string
		userID     int64
		body       string
		booking    *models.Booking
		getErr     error
		existing   []models.Booking
		updateErr  error
		wantStatus int
		wantUpdate bool
	}{
		{
			name:       "successful update",
			userID:     5,
			body:       validBookingJSON,
			booking:    ownedBooking,
			wantStatus: http.StatusOK,
			wantUpdate: true,
		},
		{
			name:       "unauthorized user",
			body:       validBookingJSON,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "booking not found",
			userID:     5,
			body:       validBookingJSON,
			getErr:     myErrors.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "another user's booking",
			userID:     5,
			body:       validBookingJSON,
			booking:    otherBooking,
			wantStatus: http.StatusForbidden,
		},
		{
			name:    "invalid interval",
			userID:  5,
			booking: ownedBooking,
			body: `{
				"room_id": 1,
				"start_time": "2026-11-18T12:00:00+03:00",
				"end_time": "2026-11-18T11:00:00+03:00"
			}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "overlap with another booking",
			userID:     5,
			body:       validBookingJSON,
			booking:    ownedBooking,
			existing:   []models.Booking{conflict},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "cancelled booking cannot be updated",
			userID:     5,
			body:       validBookingJSON,
			booking:    ownedBooking,
			updateErr:  myErrors.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantUpdate: true,
		},
		{
			name:       "repository detects overlap",
			userID:     5,
			body:       validBookingJSON,
			booking:    ownedBooking,
			updateErr:  myErrors.ErrOverlap,
			wantStatus: http.StatusConflict,
			wantUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockBookingRepository{
				booking:   tt.booking,
				getErr:    tt.getErr,
				bookings:  tt.existing,
				updateErr: tt.updateErr,
			}

			server := &Server{Bookings: repo}

			req := bookingTestRequest(
				http.MethodPut, "/bookings/7", tt.body, tt.userID,
			)
			req.SetPathValue("id", "7")

			rec := httptest.NewRecorder()
			server.updateBooking(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body = %s",
					rec.Code, tt.wantStatus, rec.Body.String(),
				)
			}

			if repo.updateCalled != tt.wantUpdate {
				t.Errorf(
					"Update called = %v, want %v",
					repo.updateCalled, tt.wantUpdate,
				)
			}
		})
	}
}

func TestCancelBooking(t *testing.T) {
	tests := []struct {
		name       string
		userID     int64
		cancelErr  error
		wantStatus int
		wantCancel bool
	}{
		{
			name:       "successful cancellation",
			userID:     5,
			wantStatus: http.StatusNoContent,
			wantCancel: true,
		},
		{
			name:       "unauthorized user",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "booking not found or belongs to another user",
			userID:     5,
			cancelErr:  myErrors.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCancel: true,
		},
		{
			name:       "repository error",
			userID:     5,
			cancelErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCancel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockBookingRepository{
				cancelErr: tt.cancelErr,
			}

			server := &Server{Bookings: repo}

			req := bookingTestRequest(
				http.MethodDelete, "/bookings/7", "", tt.userID,
			)
			req.SetPathValue("id", "7")

			rec := httptest.NewRecorder()
			server.cancelBooking(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body = %s",
					rec.Code, tt.wantStatus, rec.Body.String(),
				)
			}

			if repo.cancelCalled != tt.wantCancel {
				t.Errorf(
					"Cancel called = %v, want %v",
					repo.cancelCalled, tt.wantCancel,
				)
			}

			if tt.wantCancel {
				if repo.cancelUserID != tt.userID {
					t.Errorf(
						"Cancel userID = %d, want %d",
						repo.cancelUserID, tt.userID,
					)
				}

				if repo.cancelID != 7 {
					t.Errorf(
						"Cancel booking ID = %d, want 7",
						repo.cancelID,
					)
				}
			}

			if rec.Code == http.StatusNoContent && rec.Body.Len() != 0 {
				t.Errorf("204 response must have empty body")
			}
		})
	}
}
