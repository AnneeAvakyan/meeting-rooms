// Package storage содержит репозитории для доступа к данным.
//
// На старте используется простая in-memory реализация (потокобезопасная,
// на мьютексах), чтобы можно было поднять и протестировать бизнес-логику
// и API без БД. Далее она заменяется на PostgreSQL-репозиторий с тем же
// интерфейсом (см. BookingRepository) — остальной код менять не придётся.
package storage

import (
	"sync"

	"github.com/vv/meeting-room-booking/internal/booking"
	"github.com/vv/meeting-room-booking/internal/room"
)

// BookingRepository — интерфейс доступа к бронированиям.
// Реализуется и in-memory хранилищем, и позже — PostgreSQL-репозиторием.
type BookingRepository interface {
	Create(b booking.Booking) (booking.Booking, error)
	Update(b booking.Booking) (booking.Booking, error)
	Cancel(id int64) error
	ListByRoom(roomID int64) ([]booking.Booking, error)
	ListAll() ([]booking.Booking, error)
}

// RoomRepository — интерфейс доступа к переговорным комнатам.
type RoomRepository interface {
	List() ([]room.Room, error)
	Get(id int64) (room.Room, error)
}

// MemoryStore — потокобезопасное in-memory хранилище для разработки и тестов.
type MemoryStore struct {
	mu        sync.Mutex
	rooms     []room.Room
	bookings  []booking.Booking
	nextBkID  int64
}

// NewMemoryStore создаёт хранилище с несколькими демо-комнатами.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		rooms: []room.Room{
			{ID: 1, Name: "Переговорная 1", Capacity: 4, Equipment: "TV, маркерная доска"},
			{ID: 2, Name: "Переговорная 2", Capacity: 8, Equipment: "Проектор, видеосвязь"},
			{ID: 3, Name: "Переговорная 3", Capacity: 2, Equipment: "—"},
		},
		bookings: []booking.Booking{},
		nextBkID: 1,
	}
}

func (s *MemoryStore) List() ([]room.Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]room.Room, len(s.rooms))
	copy(out, s.rooms)
	return out, nil
}

func (s *MemoryStore) Get(id int64) (room.Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rooms {
		if r.ID == id {
			return r, nil
		}
	}
	return room.Room{}, ErrNotFound
}

func (s *MemoryStore) Create(b booking.Booking) (booking.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b.ID = s.nextBkID
	s.nextBkID++
	if b.Status == "" {
		b.Status = booking.StatusActive
	}
	s.bookings = append(s.bookings, b)
	return b, nil
}

func (s *MemoryStore) Update(b booking.Booking) (booking.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.bookings {
		if existing.ID == b.ID {
			s.bookings[i] = b
			return b, nil
		}
	}
	return booking.Booking{}, ErrNotFound
}

func (s *MemoryStore) Cancel(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.bookings {
		if existing.ID == id {
			s.bookings[i].Status = booking.StatusCancelled
			return nil
		}
	}
	return ErrNotFound
}

func (s *MemoryStore) ListByRoom(roomID int64) ([]booking.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []booking.Booking
	for _, b := range s.bookings {
		if b.RoomID == roomID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *MemoryStore) ListAll() ([]booking.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]booking.Booking, len(s.bookings))
	copy(out, s.bookings)
	return out, nil
}
