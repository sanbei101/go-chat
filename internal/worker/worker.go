package worker

import (
	"context"
	"fmt"
	"time"

	proto "github.com/sanbei101/im/pkg/protocol"
)

// Handler 用于承接持久化、未读数计算等重逻辑。
type Handler func(ctx context.Context, msg *proto.ChatMessage) (*proto.ChatMessage, error)

// Deliverer 负责将处理后的消息投递给网关服务。
type Deliverer func(ctx context.Context, msg *proto.ChatMessage) error

// Worker 负责处理网关上送的消息，并在处理后回投到网关。
type Worker struct {
	handler   Handler
	deliverer Deliverer
}

func New(handler Handler, deliverer Deliverer) *Worker {
	return &Worker{
		handler:   handler,
		deliverer: deliverer,
	}
}

func (w *Worker) Process(ctx context.Context, msg *proto.ChatMessage) (*proto.ChatMessage, error) {
	var err error
	if w.handler != nil {
		msg, err = w.handler(ctx, msg)
		if err != nil {
			return nil, err
		}
	} else {
		msg = decorateMessage(msg)
	}

	if w.deliverer != nil {
		if err := w.deliverer(ctx, msg); err != nil {
			return nil, err
		}
	}

	return msg, nil
}

func decorateMessage(msg *proto.ChatMessage) *proto.ChatMessage {
	if msg == nil {
		msg = &proto.ChatMessage{}
	}
	if msg.MsgId == "" {
		msg.MsgId = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if msg.ServerTime == 0 {
		msg.ServerTime = time.Now().UnixMilli()
	}
	return msg
}
