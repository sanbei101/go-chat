package ws

import (
	"context"
	"log"
	"time"

	"github.com/gorilla/websocket"
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
		cl.Conn.Close()
	}()

	for {
		message, ok := <-cl.Message
		if !ok {
			return
		}

		if err := cl.Conn.WriteJSON(message); err != nil {
			return
		}
	}
}

// Read message from frontend
func (cl *Client) ReadMessage(hub *Hub) {
	defer func() {
		hub.Unregister(cl)
		cl.Conn.Close()
	}()

	for {
		_, m, err := cl.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
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
