package repositoriesPostgres

import (
	"context"
	"errors"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepositoryPostgres struct {
	pool *pgxpool.Pool
}

func NewBookingRepositoryPostgres(pool *pgxpool.Pool) *BookingRepositoryPostgres {
	return &BookingRepositoryPostgres{pool}
}

func (r *BookingRepositoryPostgres) Create(ctx context.Context, newBooking *models.Booking) error {
	row := r.pool.QueryRow(ctx, `INSERT INTO bookings (user_id, status, start_time, end_time, room_id) 
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		newBooking.UserID, newBooking.Status, newBooking.StartTime, newBooking.EndTime, newBooking.RoomID)
	return row.Scan(&newBooking.ID, &newBooking.CreatedAt)
}

func (r *BookingRepositoryPostgres) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM bookings WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return myErrors.ErrNotFound
	}
	return nil
}

func (r *BookingRepositoryPostgres) Update(ctx context.Context, newBooking *models.Booking) error {
	row := r.pool.QueryRow(ctx, `UPDATE bookings
		SET user_id = $1, status = $2, start_time = $3, end_time = $4, room_id = $5
		WHERE id = $6
		RETURNING id, created_at`,
		newBooking.UserID, newBooking.Status, newBooking.StartTime, newBooking.EndTime, newBooking.RoomID)

	if err := row.Scan(&newBooking.ID, &newBooking.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return myErrors.ErrNotFound
		}
		return err
	}
	return nil
}
