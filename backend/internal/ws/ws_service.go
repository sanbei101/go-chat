package ws

import (
	"context"
	"errors"
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
		s.hub.AddRoom(room)
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

	s.hub.AddRoom(room)

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

	history, err := s.Repository.FetchRoomMessages(c, cl.RoomID)
	if err != nil {
		log.Printf("ws: fetch room messages: %v", err)
	}

	if ok := s.hub.Register(cl); !ok {
		return errors.New("room not found")
	}

	go cl.WriteMessage()

	for _, msg := range history {
		cl.Message <- msg
	}

	ctx, cancel := context.WithTimeout(c, s.timeout)
	defer cancel()

	if err := s.hub.Publish(ctx, m); err != nil {
		s.hub.Remove(cl)
		return err
	}

	cl.ReadMessage(s.hub)

	return nil
}

func (s *service) GetRooms(ctx context.Context) (r []RoomRes) {
	return s.hub.RoomsSnapshot()
}

func (s *service) GetClients(ctx context.Context, roomID string) (c []ClientRes) {
	return s.hub.ClientsSnapshot(roomID)
}
