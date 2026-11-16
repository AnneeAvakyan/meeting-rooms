package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/domain/repository"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type Server struct {
	Rooms        repository.RoomRepository
	Bookings     repository.BookingRepository
	Users        repository.UserRepository
	AuthHandlers *AuthHandlers
	Logger       *slog.Logger
}

type ErrorResponse struct {
	Error string `json:"error" example:"not found"`
}

func (s *Server) NewRouter() http.Handler {
	authenticated := func(handler http.HandlerFunc) http.Handler {
		return s.AuthHandlers.AuthMiddleware(handler)
	}

	mux := http.NewServeMux()

	mux.Handle(
		"GET /swagger/",
		httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		),
	)
	
	mux.Handle("GET /rooms", authenticated(s.listRooms))
	mux.Handle(
		"POST /rooms",
		authenticated(
			s.AdminMiddleware(
				http.HandlerFunc(s.createRoom),
			),
		),
	)
	mux.Handle("GET /rooms/available", authenticated(s.findAvailableRooms))
	mux.Handle("GET /rooms/{id}/calendar", authenticated(s.roomCalendar))     // тут получение всех броней одной комнаты
	mux.Handle("GET /rooms/{id}/active", authenticated(s.roomCalendarActive)) // а тут только активные брони

	mux.Handle("POST /bookings", authenticated(s.createBooking))
	mux.Handle("PUT /bookings/{id}", authenticated(s.updateBooking))
	mux.Handle("DELETE /bookings/{id}", authenticated(s.cancelBooking))

	mux.HandleFunc("GET /healthz", s.health)

	mux.HandleFunc("POST /auth/login", s.AuthHandlers.Login)
	mux.HandleFunc("POST /auth/register", s.AuthHandlers.Register)

	return loggingMiddleware(s.Logger, mux)
}

// health godoc
// @Summary Проверка работоспособности сервиса
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /healthz [get]
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// List rooms godoc
// @Summary Получение всех комнат
// @Description Все созданные комнаты
// @Tags Rooms
// @Produce json
// @Success 200 {array} models.Room
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /rooms [get]
func (s *Server) listRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.Rooms.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rooms)
}

type createRoomRequest struct {
	Name      string `json:"name"`
	Capacity  int    `json:"capacity"`
	Equipment string `json:"equipment"`
}

// Create room godoc
// @Summary Создание комнаты
// @Description Создание комнаты с заданными параметрами: название, вместимость (чел), оборудование. Доступно только администратору
// @Tags Rooms
// @Accept json
// @Produce json
// @Param request body createRoomRequest true "Данные комнаты"
// @Success 201 {object} models.Room
// @Failure 500 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Security BearerAuth
// @Router /rooms [post]
func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Capacity <= 0 || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, errors.New("invalid room data"))
		return
	}

	room := models.Room{
		Name:      strings.TrimSpace(req.Name),
		Capacity:  req.Capacity,
		Equipment: strings.TrimSpace(req.Equipment),
	}

	if err := s.Rooms.Create(r.Context(), &room); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

type AvailableRoomsResponse struct {
	AvailableRoomIDs []int64 `json:"available_room_ids"`
}

// Find available rooms godoc
// @Summary Поиск всех доступных комнат
// @Description Получение комнат, свободных в заданный промежуток
// @Tags Rooms
// @Produce json
// @Param from query string true "Начало интервала (RFC3339)"
// @Param to query string true "Конец интервала (RFC3339)"
// @Success 200 {object} AvailableRoomsResponse
// @Failure 500 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /rooms/available [get]
func (s *Server) findAvailableRooms(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseInterval(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	rooms, err := s.Rooms.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	candidate := models.Booking{StartTime: from, EndTime: to}

	var free []int64
	for _, rm := range rooms {
		existing, err := s.Bookings.ListByRoom(r.Context(), rm.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		c := candidate
		c.RoomID = rm.ID
		if !models.HasOverlap(c, existing) {
			free = append(free, rm.ID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"available_room_ids": free})
}

// Room calendar godoc
// @Summary Получение календаря бронирований для отдельной комнаты
// @Description Получение всех бронирований, в том числе отмененнных
// @Tags Rooms
// @Produce json
// @Param id path int true "ID комнаты"
// @Success 200 {array} models.Booking
// @Failure 500 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /rooms/{id}/calendar [get]
func (s *Server) roomCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid room id"))
		return
	}

	_, err = s.Rooms.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, myErrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, myErrors.ErrNotFound)
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	bookings, err := s.Bookings.ListByRoom(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, bookings)
}

// Room calendar active godoc
// @Summary Получение календаря активных бронирований для отдельной комнаты
// @Description Получение только активных бронирований
// @Tags Rooms
// @Produce json
// @Param id path int true "ID комнаты"
// @Success 200 {array} models.Booking
// @Failure 500 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /rooms/{id}/active [get]
func (s *Server) roomCalendarActive(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid room id"))
		return
	}

	_, err = s.Rooms.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, myErrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, myErrors.ErrNotFound)
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	bookings, err := s.Bookings.ListActiveBookingsByRoom(r.Context(), id)
	if err != nil {
		if errors.Is(err, myErrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, errors.New("no bookings for this room"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, bookings)
}

type createBookingRequest struct {
	RoomID    int64     `json:"room_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Create booking godoc
// @Summary Создание бронирования
// @Description Создание бронирования с заданными параметрами: ID комнаты, время начала, время конца. Доступно всем авторзованным пользователям
// @Tags Bookings
// @Accept json
// @Produce json
// @Param request body createBookingRequest true "Данные бронирования"
// @Success 201 {object} models.Booking
// @Failure 500 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Security BearerAuth
// @Router /bookings [post]
func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("user not authorized"))
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	b := models.Booking{
		RoomID:    req.RoomID,
		UserID:    userID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Status:    models.StatusActive,
	}
	if err := b.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}

	existing, err := s.Bookings.ListByRoom(r.Context(), b.RoomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if models.HasOverlap(b, existing) {
		writeError(w, http.StatusConflict, myErrors.ErrOverlap)
		return
	}

	if err := s.Bookings.Create(r.Context(), &b); err != nil {
		if errors.Is(err, myErrors.ErrOverlap) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// Update booking godoc
// @Summary Изменение бронирования
// @Description Изменение бронирования с заданными параметрами: ID комнаты, время начала, время конца. Доступно всем авторзованным пользователям
// @Tags Bookings
// @Accept json
// @Produce json
// @Param request body createBookingRequest true "Данные бронирования"
// @Param id path int true "ID бронирования"
// @Success 200 {object} models.Booking
// @Failure 500 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Security BearerAuth
// @Router /bookings/{id} [put]
func (s *Server) updateBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("user not authorized"))
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid service id"))
		return
	}

	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	existingBooking, err := s.Bookings.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, myErrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, myErrors.ErrNotFound)
			return
		}
		slog.Error("error getting existing booking", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if existingBooking.UserID != userID {
		writeError(w, http.StatusForbidden, errors.New("forbidden action"))
		return
	}

	b := models.Booking{
		ID:        id,
		RoomID:    req.RoomID,
		UserID:    userID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Status:    models.StatusActive,
	}
	if err := b.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}

	existing, err := s.Bookings.ListByRoom(r.Context(), b.RoomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if models.HasOverlap(b, existing) {
		writeError(w, http.StatusConflict, myErrors.ErrOverlap)
		return
	}

	if err := s.Bookings.Update(r.Context(), &b); err != nil {
		if errors.Is(err, myErrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, myErrors.ErrOverlap) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// Cancel booking godoc
// @Summary Отмена бронирования
// @Description Отмена бронирования. Доступна только создателю бронирования
// @Tags Bookings
// @Param id path int true "ID бронирования"
// @Success 204
// @Failure 500 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /bookings/{id} [delete]
func (s *Server) cancelBooking(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid service id"))
		return
	}

	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("user not authorized"))
		return
	}
	if err := s.Bookings.Cancel(r.Context(), userID, id); err != nil {
		if errors.Is(err, myErrors.ErrNotFound) {
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
	if status >= http.StatusInternalServerError {
		slog.Error("internal server error", "error", err)
		err = errors.New("internal server error")
	}

	writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
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

func (s *Server) AdminMiddleware(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, errors.New("user not authorized"))
			return
		}
		user, err := s.Users.GetByID(r.Context(), userID)
		if err != nil {
			if errors.Is(err, myErrors.ErrNotFound) {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeError(
					w,
					http.StatusUnauthorized,
					errors.New("authentication required"),
				)
				return
			}
			s.Logger.Error("error getting user", "id", userID, "err", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if user.Role != models.RoleAdmin {
			writeError(w, http.StatusForbidden, errors.New("forbidden"))
			return
		}

		next.ServeHTTP(w, r)
	}
}
