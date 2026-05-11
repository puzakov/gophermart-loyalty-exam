package domain

import "time"

type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusInvalid    OrderStatus = "INVALID"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
)

type Order struct {
	Number     string
	UserID     int64
	Status     OrderStatus
	Accrual    *int64
	UploadedAt time.Time
}

type Withdrawal struct {
	OrderNumber string
	Sum         int64
	ProcessedAt time.Time
}

type Balance struct {
	Current   int64
	Withdrawn int64
}
