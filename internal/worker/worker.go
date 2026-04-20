package worker

import (
	"context"
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phuslu/log"
	"github.com/redis/go-redis/v9"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/pkg/config"
)

const (
	BatchReadSize = 100
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
	err := s.redis.XGroupCreateMkStream(ctx, "messages:inbound", "worker_group", "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Warn().Err(err).Msg("worker consume group messages:inbound exist")
	}
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
	result, err := s.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    "worker_group",
		Consumer: "worker1",
		Streams:  []string{"messages:inbound", ">"},
		Count:    BatchReadSize,
		Block:    time.Second,
		NoAck:    false,
	}).Result()
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			log.Info().Msg("worker 收到退出信号，停止读取消息")
		case errors.Is(err, redis.Nil):
			log.Info().Msg("worker 暂无新消息，继续轮询")
		default:
			log.Error().Err(err).Msg("worker xread 读取失败")
		}
		return
	}
	if len(result) == 0 {
		log.Warn().Msg("worker xread 结果为空,继续轮询")
		return
	}
	stream := result[0]

	params := make([]db.BatchCopyMessagesParams, 0, BatchReadSize)
	msgIDs := make([]string, 0, BatchReadSize)
	msgs := make([]*db.Message, 0, BatchReadSize)

	for _, msg := range stream.Messages {
		msgIDs = append(msgIDs, msg.ID)
		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}
		var chatMsg db.Message
		if err := json.Unmarshal([]byte(data), &chatMsg); err != nil {
			log.Error().Err(err).Msg("worker unmarshal failed")
			continue
		}
		msgs = append(msgs, &chatMsg)
		params = append(params, db.BatchCopyMessagesParams{
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
	_, err = s.queries.BatchCopyMessages(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("batch insert error")
		return
	}
	if err := s.publishDeliverBatch(ctx, msgs); err != nil {
		log.Error().Err(err).Msg("worker publish deliver batch failed")
		return
	}
	s.redis.XAck(ctx, "messages:inbound", "worker_group", msgIDs...)

}

func (s *Service) publishDeliverBatch(ctx context.Context, msgs []*db.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	pipe := s.redis.Pipeline()
	for _, msg := range msgs {
		bin, err := json.Marshal(msg)
		if err != nil {
			log.Error().Err(err).Msg("worker marshal failed in publishDeliverBatch")
			continue
		}
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: "messages:deliver",
			MaxLen: 100000,
			Approx: true,
			Values: map[string]any{"data": string(bin)},
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}
