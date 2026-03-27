package main

import (
	"context"
	"os/signal"
	"sync"
	"syscall"

	"github.com/phuslu/log"
	"github.com/redis/go-redis/v9"

	"github.com/sanbei101/im/internal/worker"
	"github.com/sanbei101/im/pkg/config"
	"github.com/sanbei101/im/pkg/logger"
)

var wg sync.WaitGroup

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

	wg.Go(func() {
		svc.Run(ctx)
	})

	wg.Wait()
	log.Info().Msg("worker stopped")
}
