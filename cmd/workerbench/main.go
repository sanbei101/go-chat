package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phuslu/log"
	"github.com/redis/go-redis/v9"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/pkg/config"
)

const (
	MessageCount = 100000
	WorkerCount  = 10
	BatchSize    = 100
)

func main() {
	cfg := config.NewTest()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create pgxpool")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to ping postgres")
	}

	queries := db.New(pool)

	fmt.Printf("Worker Bench: %d messages, %d workers, batch size %d\n", MessageCount, WorkerCount, BatchSize)

	_ = rdb
	_ = queries
}
