package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/domain/repository"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
)

type mockRoomRepository struct {
	repository.RoomRepository

	rooms     []models.Room
	room      *models.Room
	listErr   error
	getErr    error
	createErr error

	listCalled   bool
	getCalled    bool
	createCalled bool
}

func (m *mockRoomRepository) List(
	ctx context.Context,
) ([]models.Room, error) {
	m.listCalled = true
	return m.rooms, m.listErr
}

func (m *mockRoomRepository) GetByID(
	ctx context.Context,
	id int64,
) (*models.Room, error) {
	m.getCalled = true
	return m.room, m.getErr
}

func (m *mockRoomRepository) Create(
	ctx context.Context,
	room *models.Room,
) error {
	m.createCalled = true

	if m.createErr != nil {
		return m.createErr
	}

	room.ID = 10
	return nil
}

type roomHandlerRoomsStub struct {
	repository.RoomRepository

	rooms     []models.Room
	room      *models.Room
	listErr   error
	getErr    error
	createErr error

	listCalls   int
	getCalls    int
	createCalls int
	createdRoom *models.Room
}

func (m *roomHandlerRoomsStub) List(
	ctx context.Context,
) ([]models.Room, error) {
	m.listCalls++
	return m.rooms, m.listErr
}

func (m *roomHandlerRoomsStub) GetByID(
	ctx context.Context,
	id int64,
) (*models.Room, error) {
	m.getCalls++
	return m.room, m.getErr
}

func (m *roomHandlerRoomsStub) Create(
	ctx context.Context,
	room *models.Room,
) error {
	m.createCalls++

	if m.createErr != nil {
		return m.createErr
	}

	room.ID = 10
	m.createdRoom = room
	return nil
}

type roomHandlerBookingsStub struct {
	repository.BookingRepository

	bookings      map[int64][]models.Booking
	active        map[int64][]models.Booking
	listErr       error
	activeErr     error
	listCalls     int
	activeCalls   int
	requestedRoom int64
}

func (m *roomHandlerBookingsStub) ListByRoom(
	ctx context.Context,
	roomID int64,
) ([]models.Booking, error) {
	m.listCalls++
	m.requestedRoom = roomID

	if m.listErr != nil {
		return nil, m.listErr
	}

	return m.bookings[roomID], nil
}

func (m *roomHandlerBookingsStub) ListActiveBookingsByRoom(
	ctx context.Context,
	roomID int64,
) ([]models.Booking, error) {
	m.activeCalls++
	m.requestedRoom = roomID

	if m.activeErr != nil {
		return nil, m.activeErr
	}

	return m.active[roomID], nil
}

func checkHTTPStatus(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	want int,
) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf(
			"HTTP status = %d, want %d; body = %s",
			rec.Code,
			want,
			rec.Body.String(),
		)
	}
}

func TestListRooms(t *testing.T) {
	tests := []struct {
		name       string
		rooms      []models.Room
		listErr    error
		wantStatus int
		wantCount  int
	}{
		{
			name: "rooms found",
			rooms: []models.Room{
				{ID: 1, Name: "Small", Capacity: 4},
				{ID: 2, Name: "Large", Capacity: 12},
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "empty room list",
			rooms:      []models.Room{},
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name:       "database error",
			listErr:    errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roomsRepo := &roomHandlerRoomsStub{
				rooms:   tt.rooms,
				listErr: tt.listErr,
			}
			server := &Server{Rooms: roomsRepo}

			req := httptest.NewRequest(
				http.MethodGet, "/rooms", nil,
			)
			rec := httptest.NewRecorder()

			server.listRooms(rec, req)
			checkHTTPStatus(t, rec, tt.wantStatus)

			if roomsRepo.listCalls != 1 {
				t.Errorf(
					"List calls = %d, want 1",
					roomsRepo.listCalls,
				)
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			var got []models.Room
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode rooms: %v", err)
			}

			if len(got) != tt.wantCount {
				t.Errorf(
					"room count = %d, want %d",
					len(got),
					tt.wantCount,
				)
			}
		})
	}
}

func TestCreateRoom(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
		wantCreate bool
	}{
		{
			name: "successful creation",
			body: `{
				"name": "  Conference Room  ",
				"capacity": 8,
				"equipment": "  Projector  "
			}`,
			wantStatus: http.StatusCreated,
			wantCreate: true,
		},
		{
			name:       "invalid JSON",
			body:       `{"name":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty room name",
			body: `{
				"name": "   ",
				"capacity": 8,
				"equipment": "Projector"
			}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "zero capacity",
			body: `{
				"name": "Room",
				"capacity": 0
			}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative capacity",
			body: `{
				"name": "Room",
				"capacity": -5
			}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "wrong capacity type",
			body: `{
				"name": "Room",
				"capacity": "eight"
			}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "repository error",
			body: `{
				"name": "Room",
				"capacity": 8
			}`,
			createErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roomsRepo := &roomHandlerRoomsStub{
				createErr: tt.createErr,
			}
			server := &Server{Rooms: roomsRepo}

			req := httptest.NewRequest(
				http.MethodPost,
				"/rooms",
				strings.NewReader(tt.body),
			)
			rec := httptest.NewRecorder()

			server.createRoom(rec, req)
			checkHTTPStatus(t, rec, tt.wantStatus)

			called := roomsRepo.createCalls > 0
			if called != tt.wantCreate {
				t.Errorf(
					"Create called = %v, want %v",
					called,
					tt.wantCreate,
				)
			}

			if tt.wantStatus != http.StatusCreated {
				return
			}

			var got models.Room
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode created room: %v", err)
			}

			if got.ID != 10 {
				t.Errorf("room ID = %d, want 10", got.ID)
			}
			if got.Name != "Conference Room" {
				t.Errorf(
					"room name = %q, want Conference Room",
					got.Name,
				)
			}
			if got.Capacity != 8 {
				t.Errorf("capacity = %d, want 8", got.Capacity)
			}
			if got.Equipment != "Projector" {
				t.Errorf(
					"equipment = %q, want Projector",
					got.Equipment,
				)
			}
		})
	}
}

func TestRoomCalendars(t *testing.T) {
	activeBooking := models.Booking{
		ID:     1,
		RoomID: 3,
		UserID: 5,
		Status: models.StatusActive,
	}

	cancelledBooking := models.Booking{
		ID:     2,
		RoomID: 3,
		UserID: 6,
		Status: models.StatusCancelled,
	}

	tests := []struct {
		name           string
		activeOnly     bool
		pathID         string
		getErr         error
		listErr        error
		allBookings    []models.Booking
		activeBookings []models.Booking
		wantStatus     int
		wantCount      int
		wantList       bool
	}{
		{
			name:   "calendar returns all bookings",
			pathID: "3",
			allBookings: []models.Booking{
				activeBooking, cancelledBooking,
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
			wantList:   true,
		},
		{
			name:       "active calendar returns only active bookings",
			activeOnly: true,
			pathID:     "3",
			activeBookings: []models.Booking{
				activeBooking,
			},
			wantStatus: http.StatusOK,
			wantCount:  1,
			wantList:   true,
		},
		{
			name:       "empty calendar",
			pathID:     "3",
			wantStatus: http.StatusOK,
			wantCount:  0,
			wantList:   true,
		},
		{
			name:       "empty active calendar",
			activeOnly: true,
			pathID:     "3",
			wantStatus: http.StatusOK,
			wantCount:  0,
			wantList:   true,
		},
		{
			name:       "calendar invalid room ID",
			pathID:     "abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "active calendar invalid room ID",
			activeOnly: true,
			pathID:     "abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "calendar room not found",
			pathID:     "999",
			getErr:     myErrors.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "active calendar room not found",
			activeOnly: true,
			pathID:     "999",
			getErr:     myErrors.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "room lookup error",
			pathID:     "3",
			getErr:     errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "active calendar room lookup error",
			activeOnly: true,
			pathID:     "3",
			getErr:     errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "calendar bookings lookup error",
			pathID:     "3",
			listErr:    errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantList:   true,
		},
		{
			name:       "active calendar bookings lookup error",
			activeOnly: true,
			pathID:     "3",
			listErr:    errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantList:   true,
		},
		{
			name:       "active calendar repository returns not found",
			activeOnly: true,
			pathID:     "3",
			listErr:    myErrors.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantList:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roomsRepo := &roomHandlerRoomsStub{
				room: &models.Room{
					ID:       3,
					Name:     "Conference Room",
					Capacity: 8,
				},
				getErr: tt.getErr,
			}

			bookingsRepo := &roomHandlerBookingsStub{
				bookings: map[int64][]models.Booking{
					3: tt.allBookings,
				},
				active: map[int64][]models.Booking{
					3: tt.activeBookings,
				},
			}

			if tt.activeOnly {
				bookingsRepo.activeErr = tt.listErr
			} else {
				bookingsRepo.listErr = tt.listErr
			}

			server := &Server{
				Rooms:    roomsRepo,
				Bookings: bookingsRepo,
			}

			path := "/rooms/" + tt.pathID + "/calendar"
			if tt.activeOnly {
				path = "/rooms/" + tt.pathID + "/active"
			}

			req := httptest.NewRequest(
				http.MethodGet, path, nil,
			)
			req.SetPathValue("id", tt.pathID)

			rec := httptest.NewRecorder()

			if tt.activeOnly {
				server.roomCalendarActive(rec, req)
			} else {
				server.roomCalendar(rec, req)
			}

			checkHTTPStatus(t, rec, tt.wantStatus)

			if tt.pathID == "abc" {
				if roomsRepo.getCalls != 0 {
					t.Error("GetByID called for invalid ID")
				}
			} else if roomsRepo.getCalls != 1 {
				t.Errorf(
					"GetByID calls = %d, want 1",
					roomsRepo.getCalls,
				)
			}

			calls := bookingsRepo.listCalls +
				bookingsRepo.activeCalls

			if tt.wantList && calls != 1 {
				t.Errorf(
					"booking list calls = %d, want 1",
					calls,
				)
			}

			if !tt.wantList && calls != 0 {
				t.Errorf(
					"booking list unexpectedly called %d times",
					calls,
				)
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			var got []models.Booking
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode bookings: %v", err)
			}

			if len(got) != tt.wantCount {
				t.Fatalf(
					"booking count = %d, want %d",
					len(got), tt.wantCount,
				)
			}

			if tt.activeOnly {
				for _, booking := range got {
					if booking.Status != models.StatusActive {
						t.Errorf(
							"active calendar contains status %q",
							booking.Status,
						)
					}
				}
			}
		})
	}
}

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name: "valid interval",
			query: url.Values{
				"from": {"2026-11-18T10:00:00+03:00"},
				"to":   {"2026-11-18T11:00:00+03:00"},
			}.Encode(),
		},
		{
			name:    "missing both parameters",
			wantErr: true,
		},
		{
			name:    "missing to",
			query:   "from=2026-11-18T10%3A00%3A00%2B03%3A00",
			wantErr: true,
		},
		{
			name:    "invalid from",
			query:   "from=invalid&to=2026-11-18T11%3A00%3A00%2B03%3A00",
			wantErr: true,
		},
		{
			name: "invalid to",
			query: url.Values{
				"from": {"2026-11-18T10:00:00+03:00"},
				"to":   {"invalid"},
			}.Encode(),
			wantErr: true,
		},
		{
			name: "equal times",
			query: url.Values{
				"from": {"2026-11-18T10:00:00+03:00"},
				"to":   {"2026-11-18T10:00:00+03:00"},
			}.Encode(),
			wantErr: true,
		},
		{
			name: "end before start",
			query: url.Values{
				"from": {"2026-11-18T12:00:00+03:00"},
				"to":   {"2026-11-18T11:00:00+03:00"},
			}.Encode(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"/rooms/available?"+tt.query,
				nil,
			)

			from, to, err := parseInterval(req)

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"parseInterval() error = %v, wantErr = %v",
					err, tt.wantErr,
				)
			}

			if err == nil && !from.Before(to) {
				t.Errorf(
					"invalid parsed interval: %v - %v",
					from, to,
				)
			}
		})
	}
}
