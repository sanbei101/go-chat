package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phuslu/log"
	"github.com/redis/go-redis/v9"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/pkg/config"
)

// ChatMessage 是 gateway 与 worker 之间通过 Redis 传递的消息格式。
type ChatMessage struct {
	MsgID       string            `json:"msg_id"`
	ClientMsgID string            `json:"client_msg_id"`
	SenderID    string            `json:"sender_id"`
	ReceiverID  string            `json:"receiver_id"`
	ChatType    string            `json:"chat_type"`
	ServerTime  int64             `json:"server_time"`
	ReplyToMsgID string           `json:"reply_to_msg_id"`
	Payload     json.RawMessage   `json:"payload"`
	Ext         map[string]string `json:"ext"`
}

type Service struct {
	redis   *redis.Client
	queries *db.Queries
	pool    *pgxpool.Pool
}

func New(cfg *config.Config, redisClient *redis.Client) *Service {
	pool, err := pgxpool.New(context.Background(), cfg.Postgres.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("worker connect postgres failed")
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("worker ping postgres failed")
	}
	return &Service{
		redis:   redisClient,
		queries: db.New(pool),
		pool:    pool,
	}
}

func (s *Service) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.pool.Close()
			return
		default:
			s.processInbound(ctx)
		}
	}
}

func (s *Service) processInbound(ctx context.Context) {
	result, err := s.redis.XRead(ctx, &redis.XReadArgs{
		Streams: []string{"messages:inbound", "$"},
		Count:   10,
		Block:   time.Second,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Error().Err(err).Msg("worker xread failed")
		time.Sleep(time.Second)
		return
	}
	if len(result) == 0 {
		return
	}

	for _, stream := range result {
		for _, msg := range stream.Messages {
			data, ok := msg.Values["data"].(string)
			if !ok {
				continue
			}
			var chatMsg ChatMessage
			if err := json.Unmarshal([]byte(data), &chatMsg); err != nil {
				log.Error().Err(err).Msg("worker unmarshal failed")
				continue
			}

			if err := s.persist(ctx, &chatMsg); err != nil {
				log.Error().Err(err).Str("msg_id", chatMsg.MsgID).Msg("worker persist failed")
				continue
			}

			if err := s.publishDeliver(ctx, &chatMsg); err != nil {
				log.Error().Err(err).Str("msg_id", chatMsg.MsgID).Msg("worker publish deliver failed")
			}
		}
	}
}

func (s *Service) persist(ctx context.Context, msg *ChatMessage) error {
	if msg.MsgID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		msg.MsgID = id.String()
	}
	if msg.ServerTime == 0 {
		msg.ServerTime = time.Now().UnixNano()
	}

	msgUUID, err := uuid.Parse(msg.MsgID)
	if err != nil {
		return err
	}

	var clientUUID uuid.UUID
	if msg.ClientMsgID != "" {
		clientUUID, _ = uuid.Parse(msg.ClientMsgID)
	}

	var senderUUID, receiverUUID uuid.UUID
	senderUUID, _ = uuid.Parse(msg.SenderID)
	receiverUUID, _ = uuid.Parse(msg.ReceiverID)

	var replyToUUID *uuid.UUID
	if msg.ReplyToMsgID != "" {
		u, _ := uuid.Parse(msg.ReplyToMsgID)
		replyToUUID = &u
	}

	ext, _ := json.Marshal(msg.Ext)

	return s.queries.CreateMessage(ctx, db.CreateMessageParams{
		MsgID:        msgUUID,
		ClientMsgID:  clientUUID,
		SenderID:     senderUUID,
		ReceiverID:   receiverUUID,
		ChatType:     toDBChatType(msg.ChatType),
		ServerTime:   msg.ServerTime,
		ReplyToMsgID: replyToUUID,
		Payload:      msg.Payload,
		Ext:          ext,
	})
}

func (s *Service) publishDeliver(ctx context.Context, msg *ChatMessage) error {
	bin, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: "messages:deliver",
		Values: map[string]any{"data": string(bin)},
	}).Err()
}

func toDBChatType(t string) db.ChatType {
	if t == "group" {
		return db.ChatTypeGroup
	}
	return db.ChatTypeSingle
}
