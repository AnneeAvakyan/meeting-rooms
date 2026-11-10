package repositoriesPostgres

import (
	"context"
	"errors"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoomRepositoryPostgres struct {
	pool *pgxpool.Pool
}

func NewRoomRepository(pool *pgxpool.Pool) *RoomRepositoryPostgres {
	return &RoomRepositoryPostgres{pool}
}

func (r *RoomRepositoryPostgres) List(ctx context.Context) ([]models.Room, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, capacity, equipment, created_at FROM rooms`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Room])
}

func (r *RoomRepositoryPostgres) GetByID(ctx context.Context, id int64) (*models.Room, error) {
	gotRoom, err := r.pool.Query(ctx, `SELECT id, name, capacity, equipment, created_at FROM rooms 
        WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	result, err := pgx.CollectExactlyOneRow(gotRoom, pgx.RowToStructByName[models.Room])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, myErrors.ErrNotFound
		}
		return nil, err
	}
	return &result, nil
}

func (r *RoomRepositoryPostgres) Create(ctx context.Context, newRoom *models.Room) error {
	row := r.pool.QueryRow(ctx, `INSERT INTO rooms (name, capacity, equipment) 
		VALUES ($1, $2, $3)
		RETURNING id, created_at`,
		newRoom.Name, newRoom.Capacity, newRoom.Equipment)
	return row.Scan(&newRoom.ID, &newRoom.CreatedAt)
}

func (r *RoomRepositoryPostgres) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return myErrors.ErrNotFound
	}
	return nil
}

func (r *RoomRepositoryPostgres) Update(ctx context.Context, newRoom *models.Room) error {
	row := r.pool.QueryRow(ctx, `UPDATE rooms
		SET name = $1, capacity = $2, equipment = $3
		WHERE id = $4
		RETURNING id, created_at`,
		newRoom.Name, newRoom.Capacity, newRoom.Equipment, newRoom.ID)

	if err := row.Scan(&newRoom.ID, &newRoom.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return myErrors.ErrNotFound
		}
		return err
	}
	return nil
}
