package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/internal/worker"
	"github.com/sanbei101/im/pkg/config"
	"github.com/sanbei101/im/pkg/logger"
)

func main() {
	logger.InitLogger()
	cfg := config.New()

	redisClient := db.NewRedis(cfg)

	svc := worker.New(cfg, redisClient)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	svc.Run(ctx)
}
