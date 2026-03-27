package main

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/sanbei101/im/internal/gateway"
	"github.com/sanbei101/im/pkg/config"
	"github.com/sanbei101/im/pkg/logger"
)

var wg sync.WaitGroup

func main() {
	logger.InitLogger()
	config := config.New()
	g := gateway.New(config)
	ctx := context.Background()

	http.HandleFunc("/ws", g.HandleUserMessage)

	wg.Go(func() {
		g.SubscribeFromWorker(ctx)
	})

	wg.Go(func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	})

	wg.Wait()
}
