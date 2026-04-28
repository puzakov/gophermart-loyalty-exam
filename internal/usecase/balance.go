package usecase

import (
	"context"
	"time"

	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
	"github.com/puzakov/gophermart-loyalty-exam/internal/storage/postgres"
)

type BalanceUsecase struct {
	balance *postgres.BalanceRepo
}

func NewBalanceUsecase(balance *postgres.BalanceRepo) *BalanceUsecase {
	return &BalanceUsecase{balance: balance}
}

func (u *BalanceUsecase) Get(ctx context.Context, userID int64) (domain.Balance, error) {
	return u.balance.Get(ctx, userID)
}

func (u *BalanceUsecase) Withdraw(ctx context.Context, userID int64, orderNumber string, sum int64, now time.Time) error {
	return u.balance.Withdraw(ctx, userID, orderNumber, sum, now)
}

func (u *BalanceUsecase) ListWithdrawals(ctx context.Context, userID int64) ([]domain.Withdrawal, error) {
	return u.balance.ListWithdrawals(ctx, userID)
}
