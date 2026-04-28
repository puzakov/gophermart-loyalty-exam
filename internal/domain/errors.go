package domain

import "errors"

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrInvalidRequest   = errors.New("invalid request")
	ErrConflict         = errors.New("conflict")
	ErrInsufficientFund = errors.New("insufficient funds")
	ErrNotFound         = errors.New("not found")
)
