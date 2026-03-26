package gateway

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/phuslu/log"
	"google.golang.org/protobuf/encoding/protojson"

	proto "github.com/sanbei101/im/pkg/protocol"
)

const (
	defaultHandshakeTimeout = 10 * time.Second
	defaultWriteTimeout     = 5 * time.Second
	defaultSendQueueSize    = 64
)

var (
	ErrUnauthorized = errors.New("gateway: unauthorized")
	ErrUserNotFound = errors.New("gateway: user connection not found")
)

// AuthFunc 用于在握手阶段完成鉴权，并返回当前连接所属的用户 ID。
type AuthFunc func(ctx context.Context, r *http.Request, token string) (string, error)

// MessageHandler 用于处理客户端上行消息，例如转发到 MQ。
type MessageHandler func(ctx context.Context, msg *proto.ChatMessage) error

// Gateway 封装 WebSocket 连接管理与消息收发逻辑。
type Gateway struct {
	auth             AuthFunc
	onMessage        MessageHandler
	acceptOptions    *websocket.AcceptOptions
	handshakeTimeout time.Duration
	writeTimeout     time.Duration
	sendQueueSize    int

	mu    sync.RWMutex
	conns map[string]map[*client]struct{}
}

type Option func(*Gateway)

type client struct {
	userID string
	conn   *websocket.Conn
	send   chan []byte
	once   sync.Once
}

// WithAcceptOptions 设置 websocket 握手参数。
func WithAcceptOptions(opts *websocket.AcceptOptions) Option {
	return func(g *Gateway) {
		g.acceptOptions = opts
	}
}

// WithHandshakeTimeout 设置握手与单次读消息的超时时间。
func WithHandshakeTimeout(timeout time.Duration) Option {
	return func(g *Gateway) {
		if timeout > 0 {
			g.handshakeTimeout = timeout
		}
	}
}

// WithWriteTimeout 设置下行写消息超时时间。
func WithWriteTimeout(timeout time.Duration) Option {
	return func(g *Gateway) {
		if timeout > 0 {
			g.writeTimeout = timeout
		}
	}
}

// WithSendQueueSize 设置单连接发送缓冲大小。
func WithSendQueueSize(size int) Option {
	return func(g *Gateway) {
		if size > 0 {
			g.sendQueueSize = size
		}
	}
}

// New 创建一个新的消息网关实例。
func New(auth AuthFunc, onMessage MessageHandler, opts ...Option) *Gateway {
	g := &Gateway{
		auth:             auth,
		onMessage:        onMessage,
		handshakeTimeout: defaultHandshakeTimeout,
		writeTimeout:     defaultWriteTimeout,
		sendQueueSize:    defaultSendQueueSize,
		conns:            make(map[string]map[*client]struct{}),
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// ServeHTTP 处理客户端 WebSocket 连接。
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.auth == nil {
		http.Error(w, "gateway auth is not configured", http.StatusInternalServerError)
		return
	}

	token := extractToken(r)
	authCtx, cancel := context.WithTimeout(context.Background(), g.handshakeTimeout)
	defer cancel()

	userID, err := g.auth(authCtx, r, token)
	if err != nil || userID == "" {
		log.Error().Err(err).Str("remote_addr", r.RemoteAddr).Msg("gateway auth failed")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, g.acceptOptions)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("gateway accept websocket failed")
		return
	}

	c := &client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, g.sendQueueSize),
	}
	g.addClient(c)
	defer g.removeClient(c)

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		g.writeLoop(c)
	}()

	readErr := g.readLoop(r.Context(), c)
	c.close(websocket.StatusNormalClosure, "")
	<-writeDone

	if readErr != nil && !isExpectedClose(readErr) {
		log.Error().Err(readErr).Str("user_id", userID).Msg("gateway read loop stopped")
	}
}

// Push 向指定用户的在线连接投递消息。
func (g *Gateway) Push(ctx context.Context, userID string, msg *proto.ChatMessage) error {
	if userID == "" {
		return ErrUserNotFound
	}

	payload, err := marshalMessage(msg)
	if err != nil {
		return err
	}

	clients := g.snapshotClients(userID)
	if len(clients) == 0 {
		return ErrUserNotFound
	}

	for _, c := range clients {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c.send <- payload:
		default:
			log.Warn().Str("user_id", userID).Msg("gateway send queue is full, closing slow connection")
			c.close(websocket.StatusPolicyViolation, "send queue full")
		}
	}

	return nil
}

// Close 关闭网关中当前所有在线连接。
func (g *Gateway) Close(status websocket.StatusCode, reason string) {
	g.mu.RLock()
	clients := make([]*client, 0)
	for _, userClients := range g.conns {
		for c := range userClients {
			clients = append(clients, c)
		}
	}
	g.mu.RUnlock()

	for _, c := range clients {
		c.close(status, reason)
	}
}

func (g *Gateway) readLoop(parent context.Context, c *client) error {
	for {
		readCtx, cancel := context.WithTimeout(parent, g.handshakeTimeout)
		_, payload, err := c.conn.Read(readCtx)
		cancel()
		if err != nil {
			return err
		}

		msg := &proto.ChatMessage{}
		if err := protojson.Unmarshal(payload, msg); err != nil {
			log.Warn().Err(err).Str("user_id", c.userID).Msg("gateway unmarshal message failed")
			continue
		}

		// 发送者身份由网关注入，避免客户端伪造。
		msg.SenderId = c.userID
		if g.onMessage == nil {
			continue
		}
		if err := g.onMessage(parent, msg); err != nil {
			log.Error().Err(err).Str("user_id", c.userID).Msg("gateway handle message failed")
		}
	}
}

func (g *Gateway) writeLoop(c *client) {
	for payload := range c.send {
		ctx, cancel := context.WithTimeout(context.Background(), g.writeTimeout)
		err := c.conn.Write(ctx, websocket.MessageText, payload)
		cancel()
		if err != nil {
			if !isExpectedClose(err) {
				log.Error().Err(err).Str("user_id", c.userID).Msg("gateway write message failed")
			}
			c.close(websocket.StatusInternalError, "write failed")
			return
		}
	}
}

func (g *Gateway) addClient(c *client) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conns[c.userID] == nil {
		g.conns[c.userID] = make(map[*client]struct{})
	}
	g.conns[c.userID][c] = struct{}{}
}

func (g *Gateway) removeClient(c *client) {
	c.close(websocket.StatusNormalClosure, "")

	g.mu.Lock()
	defer g.mu.Unlock()

	userClients := g.conns[c.userID]
	if len(userClients) == 0 {
		return
	}
	delete(userClients, c)
	if len(userClients) == 0 {
		delete(g.conns, c.userID)
	}
}

func (g *Gateway) snapshotClients(userID string) []*client {
	g.mu.RLock()
	defer g.mu.RUnlock()

	userClients := g.conns[userID]
	if len(userClients) == 0 {
		return nil
	}

	clients := make([]*client, 0, len(userClients))
	for c := range userClients {
		clients = append(clients, c)
	}
	return clients
}

func (c *client) close(status websocket.StatusCode, reason string) {
	c.once.Do(func() {
		close(c.send)
		_ = c.conn.Close(status, reason)
	})
}

func extractToken(r *http.Request) string {
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return token
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return auth
}

func marshalMessage(msg *proto.ChatMessage) ([]byte, error) {
	if msg == nil {
		return []byte("null"), nil
	}
	return protojson.Marshal(msg)
}

func isExpectedClose(err error) bool {
	return websocket.CloseStatus(err) != -1
}
