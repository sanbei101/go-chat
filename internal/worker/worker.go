package worker

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	gproto "google.golang.org/protobuf/proto"

	proto "github.com/sanbei101/im/pkg/protocol"
)

// InboundEnvelope 表示 worker 从网关拿到的待处理消息。
type InboundEnvelope struct {
	Message *proto.ChatMessage
	Binary  []byte
}

// DeliveryEnvelope 表示 worker 完成落库后生成的投递指令。
type DeliveryEnvelope struct {
	Message *proto.ChatMessage
	Binary  []byte
}

// MessageStore 负责消息持久化。
type MessageStore interface {
	Save(ctx context.Context, msg *proto.ChatMessage) error
}

// DeliveryPublisher 负责把投递指令发回网关。
type DeliveryPublisher interface {
	Publish(ctx context.Context, envelope *DeliveryEnvelope) error
}

type DeliveryPublisherFunc func(ctx context.Context, envelope *DeliveryEnvelope) error

func (f DeliveryPublisherFunc) Publish(ctx context.Context, envelope *DeliveryEnvelope) error {
	return f(ctx, envelope)
}

// Service 负责执行 worker 的核心处理链路。
type Service struct {
	store     MessageStore
	publisher DeliveryPublisher
	now       func() time.Time
	newID     func() (uuid.UUID, error)
}

func New(store MessageStore, publisher DeliveryPublisher) *Service {
	return &Service{
		store:     store,
		publisher: publisher,
		now:       time.Now,
		newID:     uuid.NewV7,
	}
}

// Process 执行“补齐字段 -> 落库 -> 生成投递指令 -> 投递回网关”。
func (s *Service) Process(ctx context.Context, envelope *InboundEnvelope) (*DeliveryEnvelope, error) {
	if envelope == nil || envelope.Message == nil {
		return nil, errors.New("worker: message is nil")
	}

	msg, err := s.decorate(envelope.Message)
	if err != nil {
		return nil, err
	}

	if s.store == nil {
		return nil, errors.New("worker: store is nil")
	}
	if err := s.store.Save(ctx, msg); err != nil {
		return nil, err
	}

	delivery := &DeliveryEnvelope{
		Message: msg,
		Binary:  envelope.Binary,
	}
	if len(delivery.Binary) == 0 {
		delivery.Binary, err = gproto.Marshal(msg)
		if err != nil {
			return nil, err
		}
	}

	if s.publisher != nil {
		if err := s.publisher.Publish(ctx, delivery); err != nil {
			return nil, err
		}
	}

	return delivery, nil
}

func (s *Service) decorate(msg *proto.ChatMessage) (*proto.ChatMessage, error) {
	if msg.MsgId == "" {
		id, err := s.newID()
		if err != nil {
			return nil, err
		}
		msg.MsgId = id.String()
	}
	if msg.ServerTime == 0 {
		msg.ServerTime = s.now().UnixNano()
	}
	return msg, nil
}
