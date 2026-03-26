package worker

import (
	"context"
	"encoding/json"
	"errors"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/sanbei101/im/internal/db"
	proto "github.com/sanbei101/im/pkg/protocol"
)

type PostgresMessageStore struct {
	queries *db.Queries
}

func NewPostgresMessageStore(queries *db.Queries) MessageStore {
	return &PostgresMessageStore{
		queries: queries,
	}
}

// Save 负责把 worker 已经标准化过的消息落库。
func (h *PostgresMessageStore) Save(ctx context.Context, msg *proto.ChatMessage) error {
	if h.queries == nil {
		return errors.New("worker: queries is nil")
	}

	params, err := buildCreateMessageParams(msg)
	if err != nil {
		return err
	}
	return h.queries.CreateMessage(ctx, params)
}

func buildCreateMessageParams(msg *proto.ChatMessage) (db.CreateMessageParams, error) {
	if msg == nil {
		return db.CreateMessageParams{}, errors.New("worker: message is nil")
	}
	if msg.GetSenderId() == "" {
		return db.CreateMessageParams{}, errors.New("worker: sender_id is required")
	}
	if msg.GetReceiverId() == "" {
		return db.CreateMessageParams{}, errors.New("worker: receiver_id is required")
	}

	payload, err := protojson.Marshal(msg)
	if err != nil {
		return db.CreateMessageParams{}, err
	}

	ext, err := json.Marshal(msg.GetExt())
	if err != nil {
		return db.CreateMessageParams{}, err
	}

	return db.CreateMessageParams{
		MsgID:        msg.GetMsgId(),
		ClientMsgID:  msg.GetClientMsgId(),
		SenderID:     msg.GetSenderId(),
		ReceiverID:   msg.GetReceiverId(),
		ChatType:     toDBChatType(msg.GetChatType()),
		ServerTime:   msg.GetServerTime(),
		ReplyToMsgID: msg.GetReplyToMsgId(),
		Payload:      payload,
		Ext:          ext,
	}, nil
}

func toDBChatType(t proto.ChatType) db.ChatType {
	switch t {
	case proto.ChatType_CHAT_TYPE_GROUP:
		return db.ChatTypeGroup
	default:
		return db.ChatTypeSingle
	}
}
