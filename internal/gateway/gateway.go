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

const (
	StreamConsumeFromBeginning = "0"
	StreamConsumeFromNewOnly   = "$"
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
			conn.Write(r.Context(), websocket.MessageText, []byte("invalid message format"))
			continue
		}

		if message.ClientMsgID == uuid.Nil {
			log.Error().Str("user_id", userID).Msg("gateway missing client_msg_id")
			conn.Write(r.Context(), websocket.MessageText, []byte("missing client_msg_id"))
			continue
		}

		message.MsgID, err = uuid.NewV7()
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway generate msg_id failed")
			conn.Write(r.Context(), websocket.MessageText, []byte("failed to generate msg_id"))
			continue
		}
		message.ServerTime = time.Now().UnixMicro()
		message.SenderID = uuid.MustParse(userID)
		bin, err := json.Marshal(message)
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway marshal message failed")
			continue
		}

		err = g.redis.XAdd(context.Background(), &redis.XAddArgs{
			Stream: "messages:inbound",
			Values: map[string]any{"data": string(bin)},
		}).Err()

		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway push message to redis failed")
			continue
		}
	}
}

func (g *Gateway) SubscribeFromWorker(ctx context.Context) {
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
				Count:    10,
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
					g.deliverToClient(ctx, receiverID, []byte(data))
				}
				if len(msgIDs) > 0 {
					g.redis.XAck(ctx, "messages:deliver", "gateway_group", msgIDs...)
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
		err := c.conn.Write(ctx, websocket.MessageText, payload)
		c.mu.Unlock()
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway write to client failed")
		}
	}
}
