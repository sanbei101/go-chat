# Worker Bench Design

## Overview

Benchmark the worker's end-to-end message processing pipeline in isolation, without gateway dependencies.

## Architecture

- **Producer**: Directly write to `messages:inbound` stream using `go-redis` XAdd
- **Consumer**: Multiple worker goroutines running the same `processInbound` logic as the real worker
- **No WebSocket/gateway dependency**

## Data Flow

```
Redis XAdd → messages:inbound → Worker Bench (read) → PostgreSQL (batch insert) → messages:deliver → (ACK)
```

## Parameters

| Param | Value |
|-------|-------|
| `MessageCount` | 100,000 (total messages) |
| `WorkerCount` | 10 (parallel goroutines) |
| `BatchSize` | 100 (per XReadGroup) |

## Implementation

### Pre-population Phase
1. Generate `MessageCount` messages with random `ClientMsgID`, `ReceiverID`, `SenderID`
2. Use Redis pipeline to XAdd all messages to `messages:inbound`

### Worker Goroutines
Each goroutine:
1. Calls `XReadGroup` with `Group: "worker_group_bench"`, `Consumer: "bench-N"`
2. For each batch:
   - Unmarshal messages
   - Batch insert to PostgreSQL via `BatchCreateMessages`
   - Publish to `messages:deliver` via pipeline XAdd
   - XAck processed message IDs
3. Track per-goroutine metrics

### Metrics
- **Total messages processed**: Sum across all goroutines
- **Throughput**: messages/sec (total / elapsed time)
- **Latency**: Time from XRead to XAck completion
- **Error counts**: Read errors, insert errors, publish errors

### Structure

```
cmd/workerbench/
  main.go
```

`main.go` will:
1. Initialize Redis and PostgreSQL connections
2. Pre-populate `messages:inbound` stream
3. Spawn `WorkerCount` worker goroutines
4. Wait for all messages to be processed
5. Report metrics and cleanup

## Error Handling

- Continue processing on individual message errors
- Log errors but don't stop the bench
- Count errors per stage for reporting

## Cleanup

After bench completes:
- Delete the consumer group `worker_group_bench`
- Optionally trim streams (handled by stream MaxLen in publish step)
