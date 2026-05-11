package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/puzakov/gophermart-loyalty-exam/internal/accrual"
	"github.com/puzakov/gophermart-loyalty-exam/internal/auth"
	"github.com/puzakov/gophermart-loyalty-exam/internal/config"
	"github.com/puzakov/gophermart-loyalty-exam/internal/httpapi"
	"github.com/puzakov/gophermart-loyalty-exam/internal/logging"
	"github.com/puzakov/gophermart-loyalty-exam/internal/storage/postgres"
	"github.com/puzakov/gophermart-loyalty-exam/internal/usecase"
	"github.com/puzakov/gophermart-loyalty-exam/internal/worker"
)

func main() {
	log := logging.New()
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}
	if cfg.DatabaseURI == "" {
		log.Error("DATABASE_URI is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.NewPool(ctx, cfg.DatabaseURI)
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := runMigrations(cfg.DatabaseURI); err != nil {
		log.Error("migrations", "err", err)
		os.Exit(1)
	}

	tokens, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTAccessTTL)
	if err != nil {
		log.Error("token manager", "err", err)
		os.Exit(1)
	}

	usersRepo := postgres.NewUsersRepo(db)
	ordersRepo := postgres.NewOrdersRepo(db)
	balanceRepo := postgres.NewBalanceRepo(db)

	authUC := usecase.NewAuthUsecase(usersRepo, tokens)
	ordersUC := usecase.NewOrdersUsecase(ordersRepo)
	balanceUC := usecase.NewBalanceUsecase(balanceRepo)

	accrualClient := accrual.NewClient(cfg.AccrualSystemAddress)
	pool := worker.NewAccrualWorkerPool(log, ordersRepo, accrualClient, cfg.AccrualWorkers, cfg.AccrualPollInterval)
	go pool.Run(ctx)

	api := httpapi.NewServer(tokens, authUC, ordersUC, balanceUC)
	srv := &http.Server{
		Addr:              cfg.RunAddress,
		Handler:           api.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("http server started", "addr", cfg.RunAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen", "err", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("shutdown complete")
}

func runMigrations(dsn string) error {
	goose.SetDialect("postgres")
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return goose.Up(db, "migrations")
}
