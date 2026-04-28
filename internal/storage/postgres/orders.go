package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
)

type OrdersRepo struct {
	db *pgxpool.Pool
}

func NewOrdersRepo(db *pgxpool.Pool) *OrdersRepo {
	return &OrdersRepo{db: db}
}

type CreateOrderResult int

const (
	CreateOrderInserted CreateOrderResult = iota
	CreateOrderExistsSameUser
	CreateOrderExistsOtherUser
)

func (r *OrdersRepo) Create(ctx context.Context, userID int64, number string, now time.Time) (CreateOrderResult, error) {
	// Use a no-op UPDATE on conflict to learn owner + whether inserted.
	var inserted bool
	var owner int64
	err := r.db.QueryRow(ctx, `
INSERT INTO orders(number, user_id, status, uploaded_at, updated_at)
VALUES ($1,$2,'NEW',$3,$3)
ON CONFLICT (number) DO UPDATE SET number = EXCLUDED.number
RETURNING (xmax = 0) AS inserted, user_id
`, number, userID, now).Scan(&inserted, &owner)
	if err != nil {
		return 0, err
	}
	if owner != userID {
		return CreateOrderExistsOtherUser, nil
	}
	if inserted {
		return CreateOrderInserted, nil
	}
	return CreateOrderExistsSameUser, nil
}

func (r *OrdersRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Order, error) {
	rows, err := r.db.Query(ctx, `
SELECT number, user_id, status, accrual, uploaded_at
FROM orders
WHERE user_id = $1
ORDER BY uploaded_at DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		var o domain.Order
		var status string
		var accrual *int64
		if err := rows.Scan(&o.Number, &o.UserID, &status, &accrual, &o.UploadedAt); err != nil {
			return nil, err
		}
		o.Status = domain.OrderStatus(status)
		o.Accrual = accrual
		out = append(out, o)
	}
	return out, rows.Err()
}

type PendingOrder struct {
	Number string
}

func (r *OrdersRepo) ListPending(ctx context.Context, limit int) ([]PendingOrder, error) {
	rows, err := r.db.Query(ctx, `
SELECT number
FROM orders
WHERE status IN ('NEW','PROCESSING')
ORDER BY uploaded_at ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingOrder
	for rows.Next() {
		var p PendingOrder
		if err := rows.Scan(&p.Number); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *OrdersRepo) SetProcessing(ctx context.Context, number string, now time.Time) error {
	_, err := r.db.Exec(ctx, `
UPDATE orders
SET status='PROCESSING', updated_at=$2
WHERE number=$1 AND status='NEW'
`, number, now)
	return err
}

type ApplyAccrualParams struct {
	Number  string
	Status  domain.OrderStatus
	Accrual *int64
	Now     time.Time
}

func (r *OrdersRepo) ApplyAccrual(ctx context.Context, p ApplyAccrualParams) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
UPDATE orders
SET status = $2,
    accrual = $3,
    updated_at = $4
WHERE number = $1
`, p.Number, string(p.Status), p.Accrual, p.Now)
	if err != nil {
		return err
	}

	if p.Status == domain.OrderStatusProcessed && p.Accrual != nil && *p.Accrual > 0 {
		// Idempotent apply: only once.
		var userID int64
		var applied bool
		err = tx.QueryRow(ctx, `SELECT user_id, accrual_applied FROM orders WHERE number=$1 FOR UPDATE`, p.Number).Scan(&userID, &applied)
		if err != nil {
			return err
		}
		if !applied {
			_, err = tx.Exec(ctx, `
INSERT INTO accounts(user_id, current, withdrawn)
VALUES ($1, $2, 0)
ON CONFLICT (user_id) DO UPDATE SET current = accounts.current + EXCLUDED.current, updated_at = $3
`, userID, *p.Accrual, p.Now)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE orders SET accrual_applied=TRUE WHERE number=$1`, p.Number)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}
