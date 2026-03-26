package gateway

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/phuslu/log"
	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"

	proto "github.com/sanbei101/im/pkg/protocol"
)

var (
	ErrUnauthorized = errors.New("gateway: unauthorized")
	ErrUserNotFound = errors.New("gateway: user connection not found")
)

type AuthFunc func(ctx context.Context, r *http.Request, token string) (string, error)

type InboundEnvelope struct {
	Message *proto.ChatMessage
	Binary  []byte
}

type Publisher interface {
	Publish(ctx context.Context, envelope *InboundEnvelope) error
}

type PublisherFunc func(ctx context.Context, envelope *InboundEnvelope) error

func (f PublisherFunc) Publish(ctx context.Context, envelope *InboundEnvelope) error {
	return f(ctx, envelope)
}

type Gateway struct {
	auth      AuthFunc
	publisher Publisher
	mu        sync.RWMutex
	conns     map[string]map[*client]struct{}
}

type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func New(auth AuthFunc, publisher Publisher) *Gateway {
	return &Gateway{
		auth:      auth,
		publisher: publisher,
		conns:     map[string]map[*client]struct{}{},
	}
}

func (g *Gateway) HandleUserMessage(w http.ResponseWriter, r *http.Request) {
	if g.auth == nil {
		http.Error(w, "gateway auth is not configured", http.StatusInternalServerError)
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token = strings.TrimSpace(auth[7:])
		} else {
			token = auth
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	userID, err := g.auth(ctx, r, token)
	cancel()
	if err != nil || userID == "" {
		log.Error().Err(err).Str("remote_addr", r.RemoteAddr).Msg("gateway auth failed")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("gateway accept websocket failed")
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
		readCtx, readCancel := context.WithTimeout(r.Context(), 10*time.Second)
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
		if g.publisher == nil {
			continue
		}
		if err := g.publisher.Publish(r.Context(), &InboundEnvelope{Message: msg, Binary: bin}); err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway publish message failed")
		}
	}
}

func (g *Gateway) HandleWorkerMessage(ctx context.Context, userID string, msg *proto.ChatMessage) error {
	g.mu.RLock()
	userConns := g.conns[userID]
	clients := make([]*client, 0, len(userConns))
	for c := range userConns {
		clients = append(clients, c)
	}
	g.mu.RUnlock()
	if len(clients) == 0 {
		return ErrUserNotFound
	}

	payload, err := protojson.Marshal(msg)
	if err != nil {
		return err
	}

	for _, c := range clients {
		c.mu.Lock()
		writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.conn.Write(writeCtx, websocket.MessageText, payload)
		cancel()
		c.mu.Unlock()
		if err != nil && websocket.CloseStatus(err) == -1 {
			log.Error().Err(err).Str("user_id", userID).Msg("gateway push message failed")
		}
	}
	return nil
}

func (g *Gateway) Close(status websocket.StatusCode, reason string) {
	g.mu.RLock()
	clients := make([]*client, 0)
	for _, userConns := range g.conns {
		for c := range userConns {
			clients = append(clients, c)
		}
	}
	g.mu.RUnlock()

	for _, c := range clients {
		c.mu.Lock()
		_ = c.conn.Close(status, reason)
		c.mu.Unlock()
	}
}
