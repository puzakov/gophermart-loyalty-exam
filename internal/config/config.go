package config

import (
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string

	JWTSecret    string
	JWTAccessTTL time.Duration

	AccrualWorkers      int
	AccrualPollInterval time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var cfg Config

	flag.StringVar(&cfg.RunAddress, "a", envOrDefault("RUN_ADDRESS", "localhost:8080"), "service listen address")
	flag.StringVar(&cfg.DatabaseURI, "d", envOrDefault("DATABASE_URI", ""), "postgres DSN")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", envOrDefault("ACCRUAL_SYSTEM_ADDRESS", "http://localhost:8081"), "accrual system base URL")

	flag.Parse()

	cfg.JWTSecret = envOrDefault("JWT_SECRET", "")
	cfg.JWTAccessTTL = envDurationOrDefault("JWT_ACCESS_TTL", 30*24*time.Hour)

	cfg.AccrualWorkers = envIntOrDefault("ACCRUAL_WORKERS", 5)
	cfg.AccrualPollInterval = envDurationOrDefault("ACCRUAL_POLL_INTERVAL", 2*time.Second)

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envDurationOrDefault(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
