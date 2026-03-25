package ws

import (
	"context"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type Client struct {
	Conn     *websocket.Conn
	Message  chan *Message
	ID       string `json:"id"`
	RoomID   string `json:"room_id"`
	Username string `json:"username"`
}

type Message struct {
	Content  string `json:"content"`
	RoomID   string `json:"room_id"`
	Username string `json:"username"`
	UserID   string `json:"user_id"`
}

// Take message from client channel
// for passing to frontend
func (cl *Client) WriteMessage() {
	defer func() {
		cl.Conn.CloseNow()
	}()

	for {
		message, ok := <-cl.Message
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := wsjson.Write(ctx, cl.Conn, message)
		cancel()
		if err != nil {
			return
		}
	}
}

// Read message from frontend
func (cl *Client) ReadMessage(hub *Hub) {
	defer func() {
		hub.Unregister(cl)
		cl.Conn.CloseNow()
	}()

	for {
		_, m, err := cl.Conn.Read(context.Background())
		if err != nil {
			status := websocket.CloseStatus(err)
			if status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway {
				log.Printf("ws: read message: %v", err)
			}
			break
		}

		msg := &Message{
			Content:  string(m),
			RoomID:   cl.RoomID,
			Username: cl.Username,
			UserID:   cl.ID,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = hub.Publish(ctx, msg)
		cancel()
		if err != nil {
			log.Printf("ws: publish message: %v", err)
			break
		}
	}
}
