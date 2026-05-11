package usecase

import (
	"context"
	"time"

	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
	"github.com/puzakov/gophermart-loyalty-exam/internal/storage/postgres"
)

type OrdersStore interface {
	Create(ctx context.Context, userID int64, number string, now time.Time) (postgres.CreateOrderResult, error)
	ListByUser(ctx context.Context, userID int64) ([]domain.Order, error)
}

type OrdersUsecase struct {
	orders OrdersStore
}

func NewOrdersUsecase(orders OrdersStore) *OrdersUsecase {
	return &OrdersUsecase{orders: orders}
}

type SubmitOrderResult int

const (
	SubmitAccepted SubmitOrderResult = iota
	SubmitAlreadyBySameUser
	SubmitAlreadyByOtherUser
)

func (u *OrdersUsecase) Submit(ctx context.Context, userID int64, number string, now time.Time) (SubmitOrderResult, error) {
	res, err := u.orders.Create(ctx, userID, number, now)
	if err != nil {
		return 0, err
	}
	switch res {
	case postgres.CreateOrderInserted:
		return SubmitAccepted, nil
	case postgres.CreateOrderExistsSameUser:
		return SubmitAlreadyBySameUser, nil
	default:
		return SubmitAlreadyByOtherUser, nil
	}
}

func (u *OrdersUsecase) List(ctx context.Context, userID int64) ([]domain.Order, error) {
	return u.orders.ListByUser(ctx, userID)
}
