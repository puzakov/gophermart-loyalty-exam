package worker

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/puzakov/gophermart-loyalty-exam/internal/accrual"
	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
	"github.com/puzakov/gophermart-loyalty-exam/internal/storage/postgres"
)

type AccrualWorkerPool struct {
	log          *slog.Logger
	orders       *postgres.OrdersRepo
	accrual      *accrual.Client
	workers      int
	pollInterval time.Duration

	mu             sync.Mutex
	rateLimitUntil time.Time
}

func NewAccrualWorkerPool(log *slog.Logger, orders *postgres.OrdersRepo, accrualClient *accrual.Client, workers int, pollInterval time.Duration) *AccrualWorkerPool {
	if workers <= 0 {
		workers = 1
	}
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	return &AccrualWorkerPool{
		log:          log,
		orders:       orders,
		accrual:      accrualClient,
		workers:      workers,
		pollInterval: pollInterval,
	}
}

func (p *AccrualWorkerPool) Run(ctx context.Context) {
	jobs := make(chan string, p.workers*2)

	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.worker(ctx, workerID, jobs)
		}(i + 1)
	}

	t := time.NewTicker(p.pollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case <-t.C:
			p.dispatch(ctx, jobs)
		}
	}
}

func (p *AccrualWorkerPool) dispatch(ctx context.Context, jobs chan<- string) {
	now := time.Now()
	p.mu.Lock()
	rlUntil := p.rateLimitUntil
	p.mu.Unlock()
	if now.Before(rlUntil) {
		return
	}

	pending, err := p.orders.ListPending(ctx, p.workers*5)
	if err != nil {
		p.log.Error("list pending orders", "err", err)
		return
	}
	for _, o := range pending {
		select {
		case jobs <- o.Number:
		default:
			return
		}
	}
}

func (p *AccrualWorkerPool) worker(ctx context.Context, workerID int, jobs <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case number, ok := <-jobs:
			if !ok {
				return
			}
			p.processOne(ctx, workerID, number)
		}
	}
}

func (p *AccrualWorkerPool) processOne(ctx context.Context, workerID int, number string) {
	now := time.Now()
	info, err := p.accrual.GetOrder(ctx, number)
	if err != nil {
		if rl, ok := err.(accrual.RateLimitError); ok {
			p.applyRateLimit(now.Add(rl.RetryAfter))
			p.log.Warn("accrual rate limited", "retry_after", rl.RetryAfter)
			return
		}
		if err == accrual.ErrNotRegistered {
			// Keep as NEW (order not yet registered in accrual system).
			return
		}
		p.log.Error("accrual get order", "order", number, "err", err)
		return
	}

	var status domain.OrderStatus
	switch info.Status {
	case "REGISTERED", "PROCESSING":
		if err := p.orders.SetProcessing(ctx, number, now); err != nil {
			p.log.Error("set processing", "order", number, "err", err)
			return
		}
		status = domain.OrderStatusProcessing
	case "INVALID":
		status = domain.OrderStatusInvalid
	case "PROCESSED":
		status = domain.OrderStatusProcessed
	default:
		// Unknown status: keep processing to retry later.
		p.log.Warn("unknown accrual status", "order", number, "status", info.Status, "worker", workerID)
		return
	}

	if err := p.orders.ApplyAccrual(ctx, postgres.ApplyAccrualParams{
		Number:  number,
		Status:  status,
		Accrual: convertAccrualToMinor(info.Accrual),
		Now:     time.Now(),
	}); err != nil {
		p.log.Error("apply accrual", "order", number, "err", err)
	}
}

func (p *AccrualWorkerPool) applyRateLimit(until time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if until.After(p.rateLimitUntil) {
		p.rateLimitUntil = until
	}
}

func convertAccrualToMinor(accrual *float64) *int64 {
	if accrual == nil {
		return nil
	}
	// переводим в центы
	v := int64(math.Round(*accrual * 100))
	return &v
}
