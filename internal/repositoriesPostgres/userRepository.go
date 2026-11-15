package repositoriesPostgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/AnneeAvakyan/meeting-rooms/domain/models"
	"github.com/AnneeAvakyan/meeting-rooms/myErrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var pgErr *pgconn.PgError

type UserRepositoryPostgres struct {
	pool *pgxpool.Pool
}

func NewUserRepositoryPostgres(pool *pgxpool.Pool) *UserRepositoryPostgres {
	return &UserRepositoryPostgres{pool: pool}
}

func (r *UserRepositoryPostgres) Create(ctx context.Context, user *models.User) error {
	row := r.pool.QueryRow(ctx, `INSERT INTO users (name, email, password_hash, role) 
	VALUES ($1, $2, $3, $4)
	RETURNING id, created_at`, user.Name, user.Email, user.PasswordHash, user.Role)

	err := row.Scan(&user.ID, &user.CreatedAt)
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return myErrors.ErrEmailExists
		}
	}
	return fmt.Errorf("create user: %w", err)
}

func (r *UserRepositoryPostgres) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, name, email, password_hash, role, created_at FROM users WHERE email = $1`, email)
	user := &models.User{}
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, myErrors.ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (r *UserRepositoryPostgres) GetByID(ctx context.Context, userID int64) (*models.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, name, email, password_hash, role, created_at FROM users WHERE id = $1`, userID)
	user := &models.User{}
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, myErrors.ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}
