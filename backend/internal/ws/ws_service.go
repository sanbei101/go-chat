package ws

import (
	"context"
	"log"
	"time"
)

type service struct {
	Repository
	hub     *Hub
	timeout time.Duration
}

func NewService(repository Repository, h *Hub) Service {
	s := &service{
		repository,
		h,
		time.Duration(2) * time.Second,
	}

	err := s.FetchRooms()
	if err != nil {
		panic(err)
	}

	return s
}

// Populate hub with rooms from database
func (s *service) FetchRooms() error {
	rooms, err := s.Repository.FetchRooms()
	if err != nil {
		return err
	}

	for _, room := range rooms {
		room.Clients = make(map[string]*Client)
		s.hub.Rooms[room.ID] = room
	}

	return nil
}

func (s *service) CreateRoom(c context.Context, req *CreateRoomReq) (*CreateRoomRes, error) {
	ctx, cancel := context.WithTimeout(c, s.timeout)
	defer cancel()

	r := &Room{
		Name: req.Name,
	}

	room, err := s.Repository.CreateRoom(ctx, r)
	if err != nil {
		return nil, err
	}

	room.Clients = make(map[string]*Client)
	s.hub.Rooms[room.ID] = room

	return &CreateRoomRes{
		ID:   room.ID,
		Name: room.Name,
	}, nil
}

func (s *service) JoinRoom(c context.Context, cl *Client, m *Message) error {
	err := s.Repository.JoinRoom(c, cl)
	if err != nil {
		return err
	}

	// Register new client through the register channel
	s.hub.Register <- cl
	// Broadcast the message
	s.hub.Broadcast <- m

	history, err := s.Repository.FetchRoomMessages(c, cl.RoomID)
	if err != nil {
		// TODO: Notify client about error
		log.Printf("Failed to fetch room messages: %v", err)
	}

	go cl.WriteMessage()

	// Send chat history to the client
	// Add a small delay to prevent race condition
	// TODO: Implement system messages to handle synchronization
	go func() {
		time.Sleep(5000 * time.Millisecond)
		for _, msg := range history {
			cl.Message <- msg
			time.Sleep(100 * time.Millisecond)
		}
	}()

	cl.ReadMessage(s.hub)

	return nil
}

func (s *service) GetRooms(ctx context.Context) (r []RoomRes) {
	var rooms []RoomRes
	for _, room := range s.hub.Rooms {
		rooms = append(rooms, RoomRes{
			ID:   room.ID,
			Name: room.Name,
		})
	}

	return rooms
}

func (s *service) GetClients(ctx context.Context, roomID string) (c []ClientRes) {
	var clients []ClientRes

	if _, ok := s.hub.Rooms[roomID]; !ok {
		clients = make([]ClientRes, 0)
		return clients
	}

	for _, c := range s.hub.Rooms[roomID].Clients {
		clients = append(clients, ClientRes{
			ID:       c.ID,
			Username: c.Username,
		})
	}

	return clients
}
