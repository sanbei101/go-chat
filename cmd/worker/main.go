package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"

	"github.com/sanbei101/im/internal/worker"
	"github.com/sanbei101/im/pkg/config"
	"github.com/sanbei101/im/pkg/logger"
)

func main() {
	logger.InitLogger()
	cfg := config.New()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	svc := worker.New(cfg, redisClient)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	svc.Run(ctx)
}
