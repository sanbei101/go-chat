package gateway

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/phuslu/log"

	"github.com/redis/go-redis/v9"
	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/pkg/config"
	"github.com/sanbei101/im/pkg/jwt"
)

const (
	StreamConsumeFromBeginning = "0"
	StreamConsumeFromNewOnly   = "$"
	batchQueueSize             = 10000
	batchFlushInterval         = 10 * time.Millisecond
)

type Gateway struct {
	mu     sync.RWMutex
	conns  map[string]map[*client]struct{}
	redis  *redis.Client
	config *config.Config
	batch  chan []byte
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

func (c *client) writePump(ctx context.Context) {
	for msg := range c.send {
		if err := c.conn.Write(ctx, websocket.MessageText, msg); err != nil {
			return
		}
	}
}

func New(config *config.Config) *Gateway {
	g := &Gateway{
		conns: map[string]map[*client]struct{}{},
		redis: redis.NewClient(&redis.Options{
			Addr:     config.Redis.Addr,
			Password: config.Redis.Password,
			DB:       config.Redis.DB,
		}),
		config: config,
		batch:  make(chan []byte, batchQueueSize*10),
	}
	go g.batchFlushLoop()
	return g
}

func (g *Gateway) batchFlushLoop() {
	ticker := time.NewTicker(batchFlushInterval)
	defer ticker.Stop()
	buf := make([][]byte, 0, batchQueueSize)
	for {
		select {
		case msg := <-g.batch:
			buf = append(buf, msg)
			if len(buf) >= batchQueueSize {
				g.flushBatch(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				g.flushBatch(buf)
				buf = buf[:0]
			}
		}
	}
}

func (g *Gateway) flushBatch(msgs [][]byte) {
	if len(msgs) == 0 {
		return
	}
	ctx := context.Background()
	pipe := g.redis.Pipeline()
	for _, bin := range msgs {
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: "messages:inbound",
			MaxLen: 100000,
			Approx: true,
			Values: map[string]any{"data": string(bin)},
		})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Error().Err(err).Int("count", len(msgs)).Msg("gateway batch flush failed")
	}
}

func (g *Gateway) HandleUserMessage(w http.ResponseWriter, r *http.Request) {
	jwtToken := r.Header.Get("Authorization")
	if jwtToken == "" {
		http.Error(w, "missing Authorization header", http.StatusUnauthorized)
		log.Error().Str("remote_addr", r.RemoteAddr).Msg("gateway missing Authorization header")
		return
	}
	if len(jwtToken) > 7 && jwtToken[:7] == "Bearer " {
		jwtToken = jwtToken[7:]
	}
	userID, err := jwt.ParseToken(jwtToken)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		log.Error().Err(err).Msg("gateway parse token failed")
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("gateway accept connection failed")
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	c := &client{
		conn: conn,
		send: make(chan []byte, 100),
	}
	g.mu.Lock()
	if g.conns[userID] == nil {
		g.conns[userID] = map[*client]struct{}{}
	}
	g.conns[userID][c] = struct{}{}
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		delete(g.conns[userID], c)
		if len(g.conns[userID]) == 0 {
			delete(g.conns, userID)
		}
		g.mu.Unlock()
		close(c.send)
	}()

	go c.writePump(context.Background())
	senderUUID, err := uuid.Parse(userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("gateway parse user_id to uuid failed")
		return
	}
	for {
		_, payload, err := conn.Read(r.Context())
		if err != nil {
			if websocket.CloseStatus(err) == -1 {
				log.Error().Err(err).Str("user_id", userID).Msg("gateway read message failed")
			}
			return
		}

		var message db.Message
		if err := json.Unmarshal(payload, &message); err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway unmarshal message failed")
			select {
			case c.send <- []byte("invalid message format"):
			default:
			}
			continue
		}

		if message.ClientMsgID == uuid.Nil {
			log.Error().Str("user_id", userID).Msg("gateway missing client_msg_id")
			select {
			case c.send <- []byte("missing client_msg_id"):
			default:
			}
			continue
		}

		message.MsgID, err = uuid.NewV7()
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway generate msg_id failed")
			select {
			case c.send <- []byte("failed to generate msg_id"):
			default:
			}
			continue
		}
		message.SenderID = senderUUID
		message.ServerTime = time.Now().UnixMicro()
		bin, err := json.Marshal(message)
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway marshal message failed")
			continue
		}
		select {
		case g.batch <- bin:
		default:
			log.Warn().Str("user_id", userID).Msg("gateway batch queue full, dropping message")
		}
	}
}

func (g *Gateway) HandleWorkerMessages(ctx context.Context) {
	err := g.redis.XGroupCreateMkStream(ctx,
		"messages:deliver",
		"gateway_group",
		StreamConsumeFromBeginning,
	).Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Warn().Err(err).Msg("gateway consumer group mkstream failed")
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			result, err := g.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    "gateway_group",
				Consumer: "gateway1",
				Streams:  []string{"messages:deliver", ">"},
				Count:    1000,
				Block:    time.Second,
				NoAck:    false,
			}).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				log.Error().Err(err).Msg("gateway xread failed")
				time.Sleep(time.Second)
				continue
			}
			for _, stream := range result {
				var msgIDs []string
				for _, msg := range stream.Messages {
					msgIDs = append(msgIDs, msg.ID)
					data, ok := msg.Values["data"].(string)
					if !ok {
						continue
					}
					var message db.Message
					if err := json.Unmarshal([]byte(data), &message); err != nil {
						log.Error().Err(err).Msg("gateway unmarshal message failed")
						continue
					}
					receiverID := message.ReceiverID.String()
					g.deliverToClient(receiverID, []byte(data))
				}
				if len(msgIDs) > 0 {
					g.redis.XAck(ctx, "messages:deliver", "gateway_group", msgIDs...)
				}
			}
		}
	}
}

func (g *Gateway) deliverToClient(userID string, payload []byte) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for c := range g.conns[userID] {
		select {
		case c.send <- payload:
		default:
			log.Warn().Str("user_id", userID).Msg("gateway client buffer full, dropping message")
		}
	}
}
