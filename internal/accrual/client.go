package accrual

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	baseURL string
	http    *resty.Client
}

type OrderInfo struct {
	Order   string `json:"order"`
	Status  string `json:"status"`
	Accrual *int64 `json:"accrual,omitempty"`
}

var (
	ErrNotRegistered = errors.New("order not registered")
	ErrRateLimited   = errors.New("rate limited")
)

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e RateLimitError) Error() string { return ErrRateLimited.Error() }

func NewClient(baseURL string) *Client {
	c := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(5 * time.Second)
	return &Client{
		baseURL: baseURL,
		http:    c,
	}
}

func (c *Client) GetOrder(ctx context.Context, number string) (OrderInfo, error) {
	var out OrderInfo
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&out).
		Get("/api/orders/" + number)
	if err != nil {
		return OrderInfo{}, err
	}

	switch resp.StatusCode() {
	case http.StatusOK:
		return out, nil
	case http.StatusNoContent:
		return OrderInfo{}, ErrNotRegistered
	case http.StatusTooManyRequests:
		ra := parseRetryAfter(resp.Header().Get("Retry-After"))
		return OrderInfo{}, RateLimitError{RetryAfter: ra}
	default:
		return OrderInfo{}, errors.New("accrual unexpected status: " + strconv.Itoa(resp.StatusCode()))
	}
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 60 * time.Second
	}
	sec, err := strconv.Atoi(v)
	if err != nil || sec <= 0 {
		return 60 * time.Second
	}
	return time.Duration(sec) * time.Second
}
