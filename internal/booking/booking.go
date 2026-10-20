package booking

import (
	"errors"
	"time"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCancelled Status = "cancelled"
)

type Booking struct {
	ID        int64     `json:"id" db:"id"`
	RoomID    int64     `json:"room_id" db:"room_id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	StartTime time.Time `json:"start_time" db:"start_time"`
	EndTime   time.Time `json:"end_time" db:"end_time"`
	Status    Status    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

var (
	ErrInvalidInterval = errors.New("start_time must be before end_time")
	ErrOverlap         = errors.New("booking overlaps with an existing one")
)

func (b Booking) Validate() error {
	if !b.StartTime.Before(b.EndTime) {
		return ErrInvalidInterval
	}
	return nil
}

func (b Booking) Overlaps(other Booking) bool {
	if b.RoomID != other.RoomID {
		return false
	}
	return b.StartTime.Before(other.EndTime) && other.StartTime.Before(b.EndTime)
}

func HasOverlap(candidate Booking, existing []Booking) bool {
	for _, e := range existing {
		if e.Status != StatusActive {
			continue
		}
		if candidate.ID != 0 && candidate.ID == e.ID {
			continue // сама себя при редактировании не считаем конфликтом
		}
		if candidate.Overlaps(e) {
			return true
		}
	}
	return false
}
