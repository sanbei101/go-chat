package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phuslu/log"
	"github.com/redis/go-redis/v9"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/pkg/config"
)

var (
	processedCount atomic.Int64
	errorCount     atomic.Int64
)

const (
	MessageCount = 100000
	WorkerCount  = 10
	BatchSize    = 100
)

func startWorkerBench(ctx context.Context, rdb *redis.Client, queries *db.Queries, workerID int) {
	consumerName := fmt.Sprintf("bench-worker-%d", workerID)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			processBatch(ctx, rdb, queries, consumerName)
		}
	}
}

func processBatch(ctx context.Context, rdb *redis.Client, queries *db.Queries, consumerName string) {
	result, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    "worker_group_bench",
		Consumer: consumerName,
		Streams:  []string{"messages:inbound", ">"},
		Count:    BatchSize,
		Block:    time.Second,
		NoAck:    false,
	}).Result()
	if err != nil {
		if err != redis.Nil {
			log.Error().Err(err).Msg("xreadgroup failed")
		}
		return
	}

	for _, stream := range result {
		var params []db.BatchCreateMessagesParams
		var msgIDs []string
		var msgs []*db.Message

		for _, msg := range stream.Messages {
			msgIDs = append(msgIDs, msg.ID)
			data, ok := msg.Values["data"].(string)
			if !ok {
				continue
			}
			var chatMsg db.Message
			if err := json.Unmarshal([]byte(data), &chatMsg); err != nil {
				log.Error().Err(err).Msg("unmarshal failed")
				continue
			}
			msgs = append(msgs, &chatMsg)
			params = append(params, db.BatchCreateMessagesParams{
				MsgID:        chatMsg.MsgID,
				ClientMsgID:  chatMsg.ClientMsgID,
				SenderID:     chatMsg.SenderID,
				ReceiverID:   chatMsg.ReceiverID,
				ChatType:     chatMsg.ChatType,
				MsgType:      chatMsg.MsgType,
				ServerTime:   chatMsg.ServerTime,
				ReplyToMsgID: chatMsg.ReplyToMsgID,
				Payload:      chatMsg.Payload,
				Ext:          chatMsg.Ext,
			})
		}

		if len(params) > 0 {
			batchResult := queries.BatchCreateMessages(ctx, params)
			var batchErr error
			batchResult.Exec(func(i int, err error) {
				if err != nil {
					batchErr = err
				}
			})
			if err := batchResult.Close(); err != nil {
				log.Error().Err(err).Msg("batch close error")
			}
			if batchErr != nil {
				log.Error().Err(batchErr).Msg("batch insert error")
				errorCount.Add(int64(len(params)))
				return
			}

			// Publish to messages:deliver
			pipe := rdb.Pipeline()
			for _, msg := range msgs {
				bin, _ := json.Marshal(msg)
				pipe.XAdd(ctx, &redis.XAddArgs{
					Stream: "messages:deliver",
					MaxLen: 100000,
					Approx: true,
					Values: map[string]any{"data": string(bin)},
				})
			}
			if _, err := pipe.Exec(ctx); err != nil {
				log.Error().Err(err).Msg("publish to deliver failed")
				errorCount.Add(int64(len(msgs)))
				return
			}

			// Ack messages
			rdb.XAck(ctx, "messages:inbound", "worker_group_bench", msgIDs...)
			processedCount.Add(int64(len(msgs)))
		}
	}
}

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

	// Setup consumer group
	if err := rdb.XGroupCreateMkStream(ctx, "messages:inbound", "worker_group_bench", "0").Err(); err != nil {
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			log.Fatal().Err(err).Msg("create consumer group failed")
		}
	}

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := range WorkerCount {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			startWorkerBench(ctx, rdb, queries, id)
		}(i)
	}

	fmt.Printf("Worker Bench: %d messages, %d workers, batch size %d\n", MessageCount, WorkerCount, BatchSize)
}
