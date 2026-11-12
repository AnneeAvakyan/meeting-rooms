package repository

import (
	"context"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
)

type BookingRepository interface {
	Create(ctx context.Context, b *models.Booking) error
	Update(ctx context.Context, b *models.Booking) error
	Cancel(ctx context.Context, id int64) error
	ListByRoom(ctx context.Context, roomID int64) ([]models.Booking, error)
	ListAll(ctx context.Context) ([]models.Booking, error)
}
