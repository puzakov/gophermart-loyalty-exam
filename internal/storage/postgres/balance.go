package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
)

type BalanceRepo struct {
	db *pgxpool.Pool
}

func NewBalanceRepo(db *pgxpool.Pool) *BalanceRepo {
	return &BalanceRepo{db: db}
}

func (r *BalanceRepo) Get(ctx context.Context, userID int64) (domain.Balance, error) {
	var b domain.Balance
	err := r.db.QueryRow(ctx, `SELECT current, withdrawn FROM accounts WHERE user_id=$1`, userID).Scan(&b.Current, &b.Withdrawn)
	if IsNoRows(err) {
		return domain.Balance{}, nil
	}
	return b, err
}

func (r *BalanceRepo) Withdraw(ctx context.Context, userID int64, orderNumber string, sum int64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Ensure account row exists and lock it.
	_, err = tx.Exec(ctx, `
INSERT INTO accounts(user_id, current, withdrawn, updated_at)
VALUES ($1, 0, 0, $2)
ON CONFLICT (user_id) DO NOTHING
`, userID, now)
	if err != nil {
		return err
	}

	var current int64
	err = tx.QueryRow(ctx, `SELECT current FROM accounts WHERE user_id=$1 FOR UPDATE`, userID).Scan(&current)
	if err != nil {
		return err
	}
	if current < sum {
		return domain.ErrInsufficientFund
	}

	_, err = tx.Exec(ctx, `
INSERT INTO withdrawals(user_id, order_number, sum, processed_at)
VALUES ($1,$2,$3,$4)
`, userID, orderNumber, sum, now)
	if err != nil {
		if IsUniqueViolation(err) {
			// Idempotent retry: if same user and same sum -> OK, otherwise conflict.
			var existingUser int64
			var existingSum int64
			err2 := tx.QueryRow(ctx, `SELECT user_id, sum FROM withdrawals WHERE order_number=$1`, orderNumber).Scan(&existingUser, &existingSum)
			if err2 != nil {
				return err // original
			}
			if existingUser == userID && existingSum == sum {
				return nil
			}
			return domain.ErrConflict
		}
		return err
	}

	_, err = tx.Exec(ctx, `
UPDATE accounts
SET current = current - $2,
    withdrawn = withdrawn + $2,
    updated_at = $3
WHERE user_id = $1
`, userID, sum, now)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *BalanceRepo) ListWithdrawals(ctx context.Context, userID int64) ([]domain.Withdrawal, error) {
	rows, err := r.db.Query(ctx, `
SELECT order_number, sum, processed_at
FROM withdrawals
WHERE user_id = $1
ORDER BY processed_at DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Withdrawal
	for rows.Next() {
		var w domain.Withdrawal
		if err := rows.Scan(&w.OrderNumber, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
