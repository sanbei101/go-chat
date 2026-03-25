package ws

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Room struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Clients map[string]*Client `json:"clients"`
}

type Hub struct {
	mu    sync.RWMutex
	Rooms map[string]*Room
	Repo  Repository
	Redis *redis.Client
}

func NewHub(repository Repository, rdb *redis.Client) *Hub {
	return &Hub{
		Rooms: make(map[string]*Room),
		Repo:  repository,
		Redis: rdb,
	}
}

func (h *Hub) Start(ctx context.Context) error {
	pubsub := h.Redis.PSubscribe(ctx, "ws:room:*")
	if _, err := pubsub.Receive(ctx); err != nil {
		return err
	}

	go func() {
		defer pubsub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-pubsub.Channel():
				if !ok {
					return
				}
				h.deliver(msg.Channel, msg.Payload)
			}
		}
	}()

	return nil
}

func (h *Hub) deliver(channel, payload string) {
	var msg Message
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		log.Printf("ws: decode redis payload: %v", err)
		return
	}

	h.fanout(strings.TrimPrefix(channel, "ws:room:"), &msg)
}

func (h *Hub) AddRoom(room *Room) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if current, ok := h.Rooms[room.ID]; ok {
		current.Name = room.Name
		return
	}

	room.Clients = make(map[string]*Client)
	h.Rooms[room.ID] = room
}

func (h *Hub) Register(cl *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.Rooms[cl.RoomID]
	if !ok {
		return false
	}

	room.Clients[cl.ID] = cl
	return true
}

func (h *Hub) Unregister(cl *Client) {
	if !h.removeClient(cl) {
		return
	}

	msg := &Message{
		Content:  "user has left the chat",
		RoomID:   cl.RoomID,
		Username: cl.Username,
		UserID:   cl.ID,
	}
	if err := h.Publish(context.Background(), msg); err != nil {
		log.Printf("ws: publish leave message: %v", err)
	}
}

func (h *Hub) Remove(cl *Client) {
	h.removeClient(cl)
}

func (h *Hub) removeClient(cl *Client) bool {
	h.mu.Lock()
	room, ok := h.Rooms[cl.RoomID]
	if !ok {
		h.mu.Unlock()
		return false
	}

	current, ok := room.Clients[cl.ID]
	if !ok || current != cl {
		h.mu.Unlock()
		return false
	}

	delete(room.Clients, cl.ID)
	close(cl.Message)
	h.mu.Unlock()

	return true
}

func (h *Hub) Publish(ctx context.Context, msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if err := h.Redis.Publish(ctx, roomChannel(msg.RoomID), body).Err(); err != nil {
		return err
	}

	go func(msg *Message) {
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.Repo.WriteMessage(writeCtx, msg); err != nil {
			log.Printf("ws: write message: %v", err)
		}
	}(msg)

	return nil
}

func (h *Hub) fanout(roomID string, msg *Message) {
	h.mu.RLock()
	room, ok := h.Rooms[roomID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	clients := make([]*Client, 0, len(room.Clients))
	for _, cl := range room.Clients {
		clients = append(clients, cl)
	}
	h.mu.RUnlock()

	for _, cl := range clients {
		select {
		case cl.Message <- msg:
		default:
			log.Printf("ws: drop slow client message user=%s room=%s", cl.ID, roomID)
		}
	}
}

func (h *Hub) RoomsSnapshot() []RoomRes {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rooms := make([]RoomRes, 0, len(h.Rooms))
	for _, room := range h.Rooms {
		rooms = append(rooms, RoomRes{
			ID:   room.ID,
			Name: room.Name,
		})
	}

	return rooms
}

func (h *Hub) ClientsSnapshot(roomID string) []ClientRes {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room, ok := h.Rooms[roomID]
	if !ok {
		return []ClientRes{}
	}

	clients := make([]ClientRes, 0, len(room.Clients))
	for _, cl := range room.Clients {
		clients = append(clients, ClientRes{
			ID:       cl.ID,
			Username: cl.Username,
		})
	}

	return clients
}

func roomChannel(roomID string) string {
	return "ws:room:" + roomID
}
