package gateway

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/phuslu/log"
	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"

	"github.com/redis/go-redis/v9"
	"github.com/sanbei101/im/pkg/config"
	"github.com/sanbei101/im/pkg/jwt"
	proto "github.com/sanbei101/im/pkg/protocol"
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
		readCancel()
		if err != nil {
			if websocket.CloseStatus(err) == -1 {
				log.Error().Err(err).Str("user_id", userID).Msg("gateway read message failed")
			}
			return
		}

		req := &proto.SendMessageRequest{}
		msg := req.GetMessage()
		if err := protojson.Unmarshal(payload, req); err != nil || msg == nil {
			msg = &proto.ChatMessage{}
			if err := protojson.Unmarshal(payload, msg); err != nil {
				log.Warn().Err(err).Str("user_id", userID).Msg("gateway decode message failed")
				continue
			}
		}

		msg.SenderId = userID
		if msg.MsgId == "" {
			id, err := uuid.NewV7()
			if err != nil {
				log.Error().Err(err).Str("user_id", userID).Msg("gateway generate msg id failed")
				continue
			}
			msg.MsgId = id.String()
		}
		if msg.ServerTime == 0 {
			msg.ServerTime = time.Now().UnixNano()
		}

		bin, err := gproto.Marshal(msg)
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway marshal message failed")
			continue
		}
		err = g.redis.XAdd(readCtx, &redis.XAddArgs{
			Stream: "messages:inbound",
			Values: bin,
		}).Err()
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway push message to redis failed")
			continue
		}
	}
}

func (g *Gateway) SubscribeFromWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			result, err := g.redis.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"messages:deliver", "$"},
				Count:   10,
				Block:   time.Second,
			}).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				log.Error().Err(err).Msg("gateway xread failed")
				time.Sleep(time.Second)
				continue
			}
			for _, stream := range result {
				for _, msg := range stream.Messages {
					var chatMsg proto.ChatMessage
					if err := gproto.Unmarshal([]byte(msg.Values["data"].(string)), &chatMsg); err != nil {
						log.Error().Err(err).Msg("gateway unmarshal failed")
						continue
					}
					g.deliverToClient(ctx, chatMsg.GetReceiverId(), &chatMsg)
				}
			}
		}
	}
}

func (g *Gateway) deliverToClient(ctx context.Context, userID string, msg *proto.ChatMessage) {
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

	payload, err := protojson.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("gateway marshal deliver message failed")
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
