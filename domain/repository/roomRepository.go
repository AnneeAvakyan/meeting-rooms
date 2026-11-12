package repository

import (
	"context"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
)

type RoomRepository interface {
	List(ctx context.Context) ([]models.Room, error)
	GetByID(ctx context.Context, id int64) (*models.Room, error)
	Create(ctx context.Context, room *models.Room) error
	Update(ctx context.Context, room *models.Room) error
	Delete(ctx context.Context, id int64) error
}
