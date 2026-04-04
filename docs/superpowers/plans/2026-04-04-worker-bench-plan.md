# Worker Bench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `cmd/workerbench/main.go` that benchmarks the worker's end-to-end message processing pipeline.

**Architecture:** Pre-populate `messages:inbound` via Redis XAdd, spawn multiple worker goroutines running the same `processInbound` logic, measure throughput and latency.

**Tech Stack:** Go, go-redis, pgx, uuid, sync/atomic, pprof

---

## File Structure

```
cmd/workerbench/
  main.go          # New file - benchmark worker processing
```

---

## Task 1: Create cmd/workerbench/main.go with constants and imports

**Files:**
- Create: `cmd/workerbench/main.go`

- [ ] **Step 1: Write the initial scaffolding**

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
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

const (
	MessageCount = 100000
	WorkerCount  = 10
	BatchSize    = 100
)

var (
	processedCount atomic.Int64
	errorCount     atomic.Int64
)
```

- [ ] **Step 2: Commit**

```bash
git add cmd/workerbench/main.go
git commit -m "feat: scaffold workerbench main.go"
```

---

## Task 2: Add main function structure with Redis and PostgreSQL setup

**Files:**
- Modify: `cmd/workerbench/main.go`

- [ ] **Step 1: Add main function with config and connection setup**

```go
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
		log.Fatal().Err(err).Msg("workerbench connect postgres failed")
	}
	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("workerbench ping postgres failed")
	}
	queries := db.New(pool)

	fmt.Printf("Worker Bench: %d messages, %d workers, batch size %d\n", MessageCount, WorkerCount, BatchSize)
}
```

- [ ] **Step 2: Run go build to verify syntax**

```bash
cd /home/sanbei/go-chat && go build ./cmd/workerbench/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/workerbench/main.go
git commit -m "feat: add main function structure with Redis/Postgres setup"
```

---

## Task 3: Add message pre-population phase

**Files:**
- Modify: `cmd/workerbench/main.go`

- [ ] **Step 1: Add pre-population function and call it in main**

Add after the connection setup in main:

```go
	// Pre-populate messages:inbound stream
	fmt.Println("Pre-populating messages:inbound stream...")
	prepopulateStart := time.Now()

	pipe := rdb.Pipeline()
	for i := range MessageCount {
		msg := db.Message{
			MsgID:       uuid.New(),
			ClientMsgID: uuid.New(),
			SenderID:    uuid.New(),
			ReceiverID:  uuid.New(),
			ChatType:    db.ChatTypeSingle,
			MsgType:     db.MsgTypeText,
			ServerTime:  time.Now(),
			Payload:     json.RawMessage(fmt.Sprintf(`{"text": "bench message %d"}`, i)),
		}
		bin, _ := json.Marshal(msg)
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: "messages:inbound",
			Values: map[string]any{"data": string(bin)},
		})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to prepopulate messages:inbound")
	}
	fmt.Printf("Pre-populated %d messages in %v\n", MessageCount, time.Since(prepopulateStart))
```

- [ ] **Step 2: Run go build to verify**

```bash
go build ./cmd/workerbench/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/workerbench/main.go
git commit -m "feat: add message pre-population phase"
```

---

## Task 4: Add worker goroutines and consumer group setup

**Files:**
- Modify: `cmd/workerbench/main.go`

- [ ] **Step 1: Add worker group setup and worker goroutine function**

Add before main():

```go
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
```

- [ ] **Step 2: Add worker spawn loop in main after prepopulate**

```go
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
```

- [ ] **Step 3: Run go build to verify**

```bash
go build ./cmd/workerbench/
```

- [ ] **Step 4: Commit**

```bash
git add cmd/workerbench/main.go
git commit -m "feat: add worker goroutines and consumer group setup"
```

---

## Task 5: Add completion detection and metrics reporting

**Files:**
- Modify: `cmd/workerbench/main.go`

- [ ] **Step 1: Add completion detection and final metrics after worker spawn**

Replace the worker spawn section with polling for completion:

```go
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
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	completed := false
	for !completed {
		select {
		case <-ticker.C:
			current := processedCount.Load()
			rate := float64(current) / time.Since(startTime).Seconds()
			fmt.Printf("Processed: %d / %d (%.2f msg/s)\n", current, MessageCount, rate)
			if current >= int64(MessageCount) {
				completed = true
			}
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n--- Bench Results ---\n")
	fmt.Printf("Total messages: %d\n", MessageCount)
	fmt.Printf("Processed: %d\n", processedCount.Load())
	fmt.Printf("Errors: %d\n", errorCount.Load())
	fmt.Printf("Elapsed: %v\n", elapsed)
	fmt.Printf("Throughput: %.2f msg/s\n", float64(MessageCount)/elapsed.Seconds())

	wg.Wait()
```

Note: There's a typo above (`sync.sync.WaitGroup`) - fix to `sync.WaitGroup`.

- [ ] **Step 2: Run go build to verify**

```bash
go build ./cmd/workerbench/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/workerbench/main.go
git commit -m "feat: add completion detection and metrics reporting"
```

---

## Task 6: Add CPU/Memory profiling

**Files:**
- Modify: `cmd/workerbench/main.go`

- [ ] **Step 1: Add pprof setup at start of main and cleanup at end**

After fmt.Printf Worker Bench line, add:

```go
	cpuFile, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal().Err(err).Msg("create cpu profile failed")
	}
	defer cpuFile.Close()
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		log.Fatal().Err(err).Msg("start cpu profile failed")
	}
	defer pprof.StopCPUProfile()
```

Before the "--- Bench Results ---" fmt.Printf, add heap profile:

```go
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
```

- [ ] **Step 2: Run go build to verify**

```bash
go build ./cmd/workerbench/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/workerbench/main.go
git commit -m "feat: add CPU/memory profiling to workerbench"
```

---

## Task 7: Final review and cleanup

**Files:**
- Review: `cmd/workerbench/main.go`

- [ ] **Step 1: Read the final file and verify completeness**

```bash
cat cmd/workerbench/main.go
```

- [ ] **Step 2: Run go vet and go build**

```bash
go vet ./cmd/workerbench/ && go build ./cmd/workerbench/
```

- [ ] **Step 3: Commit final**

```bash
git add cmd/workerbench/main.go
git commit -m "feat: complete workerbench implementation"
```

---

## Self-Review Checklist

- [ ] Spec coverage: All requirements from spec are implemented
- [ ] No placeholders: No TBD/TODO/implement later
- [ ] Type consistency: db.Message, db.BatchCreateMessagesParams match existing code
- [ ] Consumer group name: `worker_group_bench` is distinct from gateway bench's `worker_group`
- [ ] Parameters match spec: MessageCount=100000, WorkerCount=10, BatchSize=100
