package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/sanbei101/im/internal/db"
	proto "github.com/sanbei101/im/pkg/protocol"
)

type PersistHandler struct {
	queries *db.Queries
	now     func() time.Time
	newID   func() string
}

func NewPersistHandler(queries *db.Queries) Handler {
	return (&PersistHandler{
		queries: queries,
		now:     time.Now,
		newID:   func() string { return fmt.Sprintf("%d", time.Now().UnixNano()) },
	}).Handle
}

// Handle 负责补齐基础字段并把消息落库。
func (h *PersistHandler) Handle(ctx context.Context, msg *proto.ChatMessage) (*proto.ChatMessage, error) {
	if h.queries == nil {
		return nil, errors.New("worker: queries is nil")
	}

	msg = h.decorate(msg)

	params, err := buildCreateMessageParams(msg)
	if err != nil {
		return nil, err
	}
	if err := h.queries.CreateMessage(ctx, params); err != nil {
		return nil, err
	}
	return msg, nil
}

func (h *PersistHandler) decorate(msg *proto.ChatMessage) *proto.ChatMessage {
	if msg == nil {
		msg = &proto.ChatMessage{}
	}
	now := h.now()
	if msg.MsgId == "" {
		msg.MsgId = h.newID()
	}
	if msg.ServerTime == 0 {
		msg.ServerTime = now.UnixMilli()
	}
	return msg
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
	case proto.ChatType_CHAT_TYPE_ROOM:
		return db.ChatTypeRoom
	default:
		return db.ChatTypeSingle
	}
}
