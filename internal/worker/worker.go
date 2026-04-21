package worker

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phuslu/log"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/pkg/config"
)

const (
	BatchReadSize = 100
)

type Service struct {
	redis   *db.Redis
	queries *db.Queries
	pool    *pgxpool.Pool
}

func New(cfg *config.Config, redisClient *db.Redis) *Service {
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
	if err := s.redis.InitStreamGroups(ctx); err != nil {
		log.Warn().Err(err).Msg("worker consume group init failed")
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
	streamMsgs, err := s.redis.WorkerPullMessage(ctx, BatchReadSize)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info().Msg("worker 收到退出信号，停止读取消息")
			return
		}
		log.Error().Err(err).Msg("worker xread读取失败")
		return
	}
	if len(streamMsgs) == 0 {
		return
	}

	params := make([]db.BatchCopyMessagesParams, 0, len(streamMsgs))
	msgIDs := make([]string, 0, len(streamMsgs))
	msgs := make([]*db.Message, 0, len(streamMsgs))

	for _, sm := range streamMsgs {
		msgIDs = append(msgIDs, sm.ID)
		chatMsg := sm.Data
		msgs = append(msgs, chatMsg)

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

	if err := s.redis.WorkerPushMessage(ctx, msgs); err != nil {
		log.Error().Err(err).Msg("worker publish deliver batch failed")
		return
	}

	if err := s.redis.AckMessages(ctx, db.MessageSteamInbound, db.MessageWorkerGroup, msgIDs...); err != nil {
		log.Error().Err(err).Msg("worker ack messages failed")
	}
}
