package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/internal/gateway"
	"github.com/sanbei101/im/pkg/config"
	"github.com/sanbei101/im/pkg/jwt"
)

const (
	UserCount    = 5000
	MessageCount = 100
)

var (
	sentCount     atomic.Int64
	receivedCount atomic.Int64
	errCount      atomic.Int64
)

func startMockWorker(rdb *redis.Client) {
	ctx := context.Background()
	rdb.XGroupCreateMkStream(ctx, "messages:inbound", "worker_group", "0")
	for {
		res, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    "worker_group",
			Consumer: "worker1",
			Streams:  []string{"messages:inbound", ">"},
			Count:    100,
			Block:    time.Second,
		}).Result()

		if err != nil || len(res) == 0 {
			continue
		}

		pipe := rdb.Pipeline()
		var msgIDs []string
		for _, msg := range res[0].Messages {
			msgIDs = append(msgIDs, msg.ID)
			pipe.XAdd(ctx, &redis.XAddArgs{
				Stream: "messages:deliver",
				Values: msg.Values,
			})
		}
		pipe.XAck(ctx, "messages:inbound", "worker_group", msgIDs...)
		pipe.Exec(ctx)
	}

}

func main() {
	cfg := config.NewTest()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})

	gw := gateway.New(cfg)
	go gw.SubscribeFromWorker(context.Background())
	go startMockWorker(rdb)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.HandleUserMessage)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + server.URL[4:] + "/ws"

	fmt.Printf("正在生成 %d 个用户并建立连接...\n", UserCount)
	var users []uuid.UUID
	conns := make([]*websocket.Conn, UserCount)

	for range UserCount {
		u, _ := uuid.NewV7()
		users = append(users, u)
	}

	var wgConnect sync.WaitGroup
	wgConnect.Add(UserCount)

	sem := make(chan struct{}, 100)
	for i := range UserCount {
		sem <- struct{}{}
		go func(idx int) {
			defer wgConnect.Done()
			defer func() { <-sem }()

			token, _ := jwt.GenerateToken(users[idx].String())
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
				HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}},
			})
			if err != nil {
				errCount.Add(1)
				return
			}
			conns[idx] = c
			go func(conn *websocket.Conn) {
				for {
					_, _, err := conn.Read(context.Background())
					if err != nil {
						return
					}
					receivedCount.Add(1)
				}
			}(c)
		}(i)
	}
	wgConnect.Wait()
	fmt.Printf("成功建立 %d 个连接，失败 %d 个\n", UserCount-int(errCount.Load()), errCount.Load())

	fmt.Println("开始压测：互相发送消息...")
	startTime := time.Now()
	var wgSend sync.WaitGroup
	wgSend.Add(UserCount)

	for i := range UserCount {
		go func(senderIdx int) {
			defer wgSend.Done()
			conn := conns[senderIdx]
			if conn == nil {
				return
			}

			for j := range MessageCount {
				receiverIdx := rand.Intn(UserCount)

				msg := db.Message{
					ClientMsgID: uuid.New(),
					ReceiverID:  users[receiverIdx],
					ChatType:    db.ChatTypeSingle,
					Payload:     json.RawMessage(fmt.Sprintf(`{"text": "Hello from user %d to user %d, msg %d"}`, senderIdx, receiverIdx, j)),
				}
				bin, _ := json.Marshal(msg)

				err := conn.Write(context.Background(), websocket.MessageText, bin)
				if err != nil {
					errCount.Add(1)
				} else {
					sentCount.Add(1)
				}
				time.Sleep(1000 * time.Millisecond)
			}
		}(i)
	}

	wgSend.Wait()

	time.Sleep(2 * time.Second)

	elapsed := time.Since(startTime)
	fmt.Printf("--- 压测结果 ---\n")
	fmt.Printf("耗时: %v\n", elapsed)
	fmt.Printf("发送消息: %d\n", sentCount.Load())
	fmt.Printf("接收消息: %d\n", receivedCount.Load())
	fmt.Printf("错误数量: %d\n", errCount.Load())
	fmt.Printf("发送 QPS: %.2f msg/s\n", float64(sentCount.Load())/elapsed.Seconds())
	fmt.Printf("接收 QPS: %.2f msg/s\n", float64(receivedCount.Load())/elapsed.Seconds())
}
