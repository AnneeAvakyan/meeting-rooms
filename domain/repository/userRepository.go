package repository

import (
	"context"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
)

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
}
