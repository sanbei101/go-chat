package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/phuslu/log"

	"github.com/sanbei101/im/pkg/config"
	"github.com/sanbei101/im/pkg/logger"
)

func main() {
	logger.InitLogger()

	cfgPath := os.Getenv("CONFIG_PATH")
	app, cleanup, err := initializeGatewayApp(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Str("config_path", configPathOrDefault(cfgPath)).Msg("initialize gateway app failed")
	}
	defer cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal().Err(err).Msg("gateway stopped unexpectedly")
	}
}

func configPathOrDefault(path string) string {
	if path != "" {
		return path
	}
	return config.DefaultConfigPath
}
