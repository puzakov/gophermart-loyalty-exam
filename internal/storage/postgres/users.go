package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
)

type UsersRepo struct {
	db *pgxpool.Pool
}

func NewUsersRepo(db *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{db: db}
}

func (r *UsersRepo) Create(ctx context.Context, login, passwordHash string) (userID int64, err error) {
	err = r.db.QueryRow(ctx,
		`INSERT INTO users(login, password_hash) VALUES ($1,$2) RETURNING id`,
		login, passwordHash,
	).Scan(&userID)
	if IsUniqueViolation(err) {
		return 0, domain.ErrConflict
	}
	return userID, err
}

func (r *UsersRepo) GetByLogin(ctx context.Context, login string) (userID int64, passwordHash string, err error) {
	err = r.db.QueryRow(ctx, `SELECT id, password_hash FROM users WHERE login=$1`, login).Scan(&userID, &passwordHash)
	if IsNoRows(err) {
		return 0, "", domain.ErrNotFound
	}
	return userID, passwordHash, err
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
