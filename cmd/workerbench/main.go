package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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

	fmt.Println("Pre-populating messages:inbound stream...")
	prepopulateStart := time.Now()
	pipe := rdb.Pipeline()
	for i := 0; i < MessageCount; i++ {
		msg := db.Message{
			MsgID:       uuid.New(),
			ClientMsgID: uuid.New(),
			SenderID:    uuid.New(),
			ReceiverID:  uuid.New(),
			ChatType:    db.ChatTypeSingle,
			MsgType:     db.MessageTypeText,
			ServerTime:  time.Now().UnixNano(),
			Payload:     json.RawMessage(fmt.Sprintf(`{"text": "bench message %d"}`, i)),
		}
		bin, _ := json.Marshal(msg)
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: "messages:inbound",
			Values: map[string]any{"data": string(bin)},
		})
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to prepopulate messages")
	}
	fmt.Printf("Pre-populated %d messages in %v\n", MessageCount, time.Since(prepopulateStart))

	fmt.Printf("Worker Bench: %d messages, %d workers, batch size %d\n", MessageCount, WorkerCount, BatchSize)

	_ = rdb
	_ = queries
}
