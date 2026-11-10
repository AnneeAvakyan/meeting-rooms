package repository

import (
	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
)

type BookingRepository interface {
	Create(b models.Booking) (models.Booking, error)
	Update(b models.Booking) (models.Booking, error)
	Cancel(id int64) error
	ListByRoom(roomID int64) ([]models.Booking, error)
	ListAll() ([]models.Booking, error)
}
