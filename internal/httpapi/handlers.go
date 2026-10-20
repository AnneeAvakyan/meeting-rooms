package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/vv/meeting-room-booking/internal/booking"
	"github.com/vv/meeting-room-booking/internal/storage"
)

type Server struct {
	Rooms    storage.RoomRepository
	Bookings storage.BookingRepository
	Logger   *slog.Logger
}

func (s *Server) NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /rooms", s.listRooms)
	mux.HandleFunc("GET /rooms/available", s.findAvailableRooms)
	mux.HandleFunc("GET /rooms/{id}/calendar", s.roomCalendar)

	mux.HandleFunc("POST /bookings", s.createBooking)
	mux.HandleFunc("PUT /bookings/{id}", s.updateBooking)
	mux.HandleFunc("DELETE /bookings/{id}", s.cancelBooking)

	mux.HandleFunc("GET /healthz", s.health)

	return loggingMiddleware(s.Logger, mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.Rooms.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rooms)
}

func (s *Server) findAvailableRooms(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseInterval(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	rooms, err := s.Rooms.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	candidate := booking.Booking{StartTime: from, EndTime: to}

	var free []int64
	for _, rm := range rooms {
		existing, err := s.Bookings.ListByRoom(rm.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		c := candidate
		c.RoomID = rm.ID
		if !booking.HasOverlap(c, existing) {
			free = append(free, rm.ID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"available_room_ids": free})
}

func (s *Server) roomCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid room id"))
		return
	}
	bookings, err := s.Bookings.ListByRoom(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, bookings)
}

type createBookingRequest struct {
	RoomID    int64     `json:"room_id"`
	UserID    int64     `json:"user_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	b := booking.Booking{
		RoomID:    req.RoomID,
		UserID:    req.UserID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Status:    booking.StatusActive,
	}
	if err := b.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}

	existing, err := s.Bookings.ListByRoom(b.RoomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if booking.HasOverlap(b, existing) {
		writeError(w, http.StatusConflict, booking.ErrOverlap)
		return
	}

	created, err := s.Bookings.Create(b)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) updateBooking(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid booking id"))
		return
	}

	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	b := booking.Booking{
		ID:        id,
		RoomID:    req.RoomID,
		UserID:    req.UserID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Status:    booking.StatusActive,
	}
	if err := b.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}

	existing, err := s.Bookings.ListByRoom(b.RoomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if booking.HasOverlap(b, existing) {
		writeError(w, http.StatusConflict, booking.ErrOverlap)
		return
	}

	updated, err := s.Bookings.Update(b)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) cancelBooking(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid booking id"))
		return
	}
	if err := s.Bookings.Cancel(id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseInterval(r *http.Request) (time.Time, time.Time, error) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, errors.New("from and to query params are required (RFC3339)")
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid 'from': " + err.Error())
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid 'to': " + err.Error())
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, errors.New("'from' must be before 'to'")
	}
	return from, to, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
		)
	})
}
