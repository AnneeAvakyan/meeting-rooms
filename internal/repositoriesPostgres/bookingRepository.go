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

func (r *BookingRepositoryPostgres) GetByID(ctx context.Context, id int64) (*models.Booking, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, room_id, user_id, start_time, end_time, status, created_at 
	FROM bookings 
	WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	result, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[models.Booking])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, myErrors.ErrNotFound
		}
		return nil, err
	}

	return &result, nil
}

func (r *BookingRepositoryPostgres) Create(ctx context.Context, newBooking *models.Booking) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var roomID int64

	if err := tx.QueryRow(ctx,
		`SELECT id FROM rooms WHERE id = $1 FOR UPDATE`,
		newBooking.RoomID,
	).Scan(&roomID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return myErrors.ErrNotFound
		}
		return err
	}

	rows, err := tx.Query(ctx, `SELECT id FROM bookings
	WHERE room_id = $1 AND status = 'active'
	AND start_time < $3 AND end_time > $2
	FOR UPDATE`, newBooking.RoomID, newBooking.StartTime, newBooking.EndTime)

	if err != nil {
		return err
	}

	conflicts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
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

func (r *BookingRepositoryPostgres) Cancel(ctx context.Context, userID int64, id int64) error {
	tag, err := r.pool.Exec(ctx, `UPDATE bookings 
	SET status = 'cancelled' 
	WHERE id = $1 
	AND status = 'active'
	AND user_id = $2`, id, userID)
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

	var roomID int64

	if err := tx.QueryRow(ctx,
		`SELECT id FROM rooms WHERE id = $1 FOR UPDATE`,
		newBooking.RoomID,
	).Scan(&roomID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return myErrors.ErrNotFound
		}
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

	conflicts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}

	if len(conflicts) > 0 {
		return myErrors.ErrOverlap
	}

	row := tx.QueryRow(ctx, `UPDATE bookings 
	SET status = $2, start_time = $3, end_time = $4, room_id = $5
	WHERE id = $6
	AND user_id = $1
	AND status = 'active'
	RETURNING id, created_at`,
		newBooking.UserID, newBooking.Status, newBooking.StartTime, newBooking.EndTime, newBooking.RoomID, newBooking.ID)

	if err := row.Scan(&newBooking.ID, &newBooking.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return myErrors.ErrNotFound
		}
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
