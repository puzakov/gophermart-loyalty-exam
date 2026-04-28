package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokensRepo struct {
	db *pgxpool.Pool
}

func NewRefreshTokensRepo(db *pgxpool.Pool) *RefreshTokensRepo {
	return &RefreshTokensRepo{db: db}
}

func (r *RefreshTokensRepo) Store(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO refresh_tokens(user_id, token_hash, expires_at) VALUES ($1,$2,$3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (r *RefreshTokensRepo) Consume(ctx context.Context, tokenHash string, now time.Time) (userID int64, err error) {
	// One-time token: lock row, mark revoked, return user_id.
	err = r.db.QueryRow(ctx, `
UPDATE refresh_tokens
SET revoked_at = $2
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > $2
RETURNING user_id
`, tokenHash, now).Scan(&userID)
	return userID, err
}
