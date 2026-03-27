package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phuslu/log"
	"github.com/redis/go-redis/v9"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/pkg/config"
)

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
			var chatMsg db.Message
			if err := json.Unmarshal([]byte(data), &chatMsg); err != nil {
				log.Error().Err(err).Msg("worker unmarshal failed")
				continue
			}

			if err := s.persist(ctx, &chatMsg); err != nil {
				log.Error().Err(err).Str("msg_id", chatMsg.MsgID.String()).Msg("worker persist failed")
				continue
			}

			if err := s.publishDeliver(ctx, &chatMsg); err != nil {
				log.Error().Err(err).Str("msg_id", chatMsg.MsgID.String()).Msg("worker publish deliver failed")
			}
		}
	}
}

func (s *Service) persist(ctx context.Context, msg *db.Message) error {
	return s.queries.CreateMessage(ctx, db.CreateMessageParams{
		MsgID:        msg.MsgID,
		ClientMsgID:  msg.ClientMsgID,
		SenderID:     msg.SenderID,
		ReceiverID:   msg.ReceiverID,
		ChatType:     msg.ChatType,
		ServerTime:   msg.ServerTime,
		ReplyToMsgID: msg.ReplyToMsgID,
		Payload:      msg.Payload,
		Ext:          msg.Ext,
	})
}

func (s *Service) publishDeliver(ctx context.Context, msg *db.Message) error {
	bin, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: "messages:deliver",
		Values: map[string]any{"data": string(bin)},
	}).Err()
}
