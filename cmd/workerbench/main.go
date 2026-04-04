package main

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"os"
	"runtime/pprof"
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
	MessageCount = 500000
	WorkerCount  = 10
	BatchSize    = 1000
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
		params := make([]db.BatchCreateMessagesParams, 0, len(stream.Messages))
		msgIDs := make([]string, 0, len(stream.Messages))
		msgs := make([]*db.Message, 0, len(stream.Messages))

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create pgxpool")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to ping postgres")
	}

	queries := db.New(pool)

	cpuFile, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal().Err(err).Msg("create cpu profile failed")
	}
	defer cpuFile.Close()
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		log.Fatal().Err(err).Msg("start cpu profile failed")
	}
	defer pprof.StopCPUProfile()

	fmt.Println("Pre-populating messages:inbound stream...")
	prepopulateStart := time.Now()
	pipe := rdb.Pipeline()
	for i := range MessageCount {
		msgID, _ := uuid.NewV7()
		clientID, _ := uuid.NewV7()
		senderID, _ := uuid.NewV7()
		receiverID, _ := uuid.NewV7()
		msg := db.Message{
			MsgID:       msgID,
			ClientMsgID: clientID,
			SenderID:    senderID,
			ReceiverID:  receiverID,
			ChatType:    db.ChatTypeSingle,
			MsgType:     db.MessageTypeText,
			ServerTime:  time.Now().UnixNano(),
			Payload:     jsontext.Value(fmt.Sprintf(`{"text": "bench message %d"}`, i)),
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

	// Poll for completion
	fmt.Println("Waiting for processing to complete...")
	startTime := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastProcessed int64
	for {
		<-ticker.C
		currentProcessed := processedCount.Load()
		currentErrors := errorCount.Load()
		log.Info().
			Int64("processed", currentProcessed).
			Int64("errors", currentErrors).
			Int64("处理速率 msg/s", currentProcessed-lastProcessed).
			Msg("当前速率")
		lastProcessed = currentProcessed
		if currentProcessed+currentErrors >= int64(MessageCount) {
			cancel()
			break
		}
	}

	elapsed := time.Since(startTime)

	pprof.StopCPUProfile()

	memFile, err := os.Create("mem.prof")
	if err != nil {
		log.Error().Err(err).Msg("create mem profile failed")
	} else {
		if err := pprof.WriteHeapProfile(memFile); err != nil {
			log.Error().Err(err).Msg("write heap profile failed")
		}
		memFile.Close()
	}

	fmt.Printf("\n--- Bench Results ---\n")
	fmt.Printf("Total messages: %d\n", MessageCount)
	fmt.Printf("Processed: %d\n", processedCount.Load())
	fmt.Printf("Errors: %d\n", errorCount.Load())
	fmt.Printf("Elapsed: %v\n", elapsed)
	wg.Wait()

	fmt.Printf("Worker Bench: %d messages, %d workers, batch size %d\n", MessageCount, WorkerCount, BatchSize)
}
