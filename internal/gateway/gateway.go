package gateway

import (
	"context"
	"encoding/json"
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

type Gateway struct {
	mu     sync.RWMutex
	conns  map[string]map[*client]struct{}
	redis  *redis.Client
	config *config.Config
}

type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func New(config *config.Config) *Gateway {
	return &Gateway{
		conns: map[string]map[*client]struct{}{},
		redis: redis.NewClient(&redis.Options{
			Addr:     config.Redis.Addr,
			Password: config.Redis.Password,
			DB:       config.Redis.DB,
		}),
		config: config,
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

	c := &client{conn: conn}
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
	}()

	for {
		readCtx, readCancel := context.WithTimeout(r.Context(), time.Duration(g.config.Gateway.MaxTimeout)*time.Second)
		_, payload, err := conn.Read(readCtx)
		if err != nil {
			readCancel()
			if websocket.CloseStatus(err) == -1 {
				log.Error().Err(err).Str("user_id", userID).Msg("gateway read message failed")
			}
			return
		}

		var message db.Message
		if err := json.Unmarshal(payload, &message); err != nil {
			readCancel()
			log.Error().Err(err).Str("user_id", userID).Msg("gateway unmarshal message failed")
			conn.Write(readCtx, websocket.MessageText, []byte("invalid message format"))
			continue
		}

		if message.ClientMsgID == uuid.Nil {
			readCancel()
			log.Error().Str("user_id", userID).Msg("gateway missing client_msg_id")
			conn.Write(readCtx, websocket.MessageText, []byte("missing client_msg_id"))
			continue
		}

		message.MsgID, err = uuid.NewV7()
		if err != nil {
			readCancel()
			log.Error().Err(err).Str("user_id", userID).Msg("gateway generate msg_id failed")
			conn.Write(readCtx, websocket.MessageText, []byte("failed to generate msg_id"))
			continue
		}
		message.ServerTime = time.Now().UnixMicro()
		bin, err := json.Marshal(message)
		if err != nil {
			readCancel()
			log.Error().Err(err).Str("user_id", userID).Msg("gateway marshal message failed")
			continue
		}

		// 使用新的 context 而非已 canceled 的 readCtx
		pubCtx, pubCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = g.redis.XAdd(pubCtx, &redis.XAddArgs{
			Stream: "messages:inbound",
			Values: map[string]any{"data": string(bin)},
		}).Err()
		pubCancel()
		readCancel()

		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway push message to redis failed")
			continue
		}
	}
}

func (g *Gateway) SubscribeFromWorker(ctx context.Context) {
	// 使用 "0" 起始 ID 而非 "$"，确保不错过任何消息
	streamID := "0"
	for {
		select {
		case <-ctx.Done():
			return
		default:
			result, err := g.redis.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"messages:deliver", streamID},
				Count:   10,
				Block:   time.Second,
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
				for _, msg := range stream.Messages {
					// 更新 streamID 为下一条消息的起始位置
					streamID = msg.ID
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
					g.deliverToClient(ctx, receiverID, []byte(data))
				}
			}
		}
	}
}

func (g *Gateway) deliverToClient(ctx context.Context, userID string, payload []byte) {
	g.mu.RLock()
	userConns := g.conns[userID]
	clients := make([]*client, 0, len(userConns))
	for c := range userConns {
		clients = append(clients, c)
	}
	g.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	for _, c := range clients {
		c.mu.Lock()
		writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.conn.Write(writeCtx, websocket.MessageText, payload)
		cancel()
		c.mu.Unlock()
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway write to client failed")
		}
	}
}
