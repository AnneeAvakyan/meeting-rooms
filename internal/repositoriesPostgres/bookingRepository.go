package repositoriesPostgres

import (
	"context"

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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT id FROM rooms WHERE id = $1 FOR UPDATE`, newBooking.RoomID); err != nil {
		return err
	}

	rows, err := tx.Query(ctx, `SELECT id FROM bookings
	WHERE room_id = $1 AND status = 'active'
	AND start_time < $3 AND end_time > $2
	FOR UPDATE`, newBooking.RoomID, newBooking.StartTime, newBooking.EndTime)

	if err != nil {
		return err
	}

	conflicts, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Booking])
	if err != nil {
		return err
	}

	if len(conflicts) > 0 {
		return myErrors.ErrOverlap
	}

	row := tx.QueryRow(ctx, `INSERT INTO bookings (user_id, status, start_time, end_time, room_id) 
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		newBooking.UserID, newBooking.Status, newBooking.StartTime, newBooking.EndTime, newBooking.RoomID)

	if err := row.Scan(&newBooking.ID, &newBooking.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *BookingRepositoryPostgres) Cancel(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `UPDATE bookings SET status = 'cancelled' WHERE id = $1 AND status = 'active'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return myErrors.ErrNotFound
	}
	return nil
}

func (r *BookingRepositoryPostgres) Update(ctx context.Context, newBooking *models.Booking) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT id FROM rooms WHERE id = $1 FOR UPDATE`, newBooking.RoomID); err != nil {
		return err
	}

	rows, err := tx.Query(ctx, `SELECT id FROM bookings
	WHERE room_id = $1 AND status = 'active'
	AND start_time < $3 AND end_time > $2
	AND  id != $4
	FOR UPDATE`, newBooking.RoomID, newBooking.StartTime, newBooking.EndTime, newBooking.ID)

	if err != nil {
		return err
	}

	conflicts, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Booking])
	if err != nil {
		return err
	}

	if len(conflicts) > 0 {
		return myErrors.ErrOverlap
	}

	row := tx.QueryRow(ctx, `UPDATE bookings 
	SET user_id = $1, status = $2, start_time = $3, end_time = $4, room_id = $5
	WHERE id = $6
	RETURNING id, created_at`,
		newBooking.UserID, newBooking.Status, newBooking.StartTime, newBooking.EndTime, newBooking.RoomID, newBooking.ID)

	if err := row.Scan(&newBooking.ID, &newBooking.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *BookingRepositoryPostgres) ListByRoom(ctx context.Context, id int64) ([]models.Booking, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, user_id, status, start_time, end_time, room_id, created_at FROM bookings WHERE room_id = $1`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Booking])
}

func (r *BookingRepositoryPostgres) ListAll(ctx context.Context) ([]models.Booking, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, user_id, status, start_time, end_time, room_id, created_at  FROM bookings`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Booking])
}
