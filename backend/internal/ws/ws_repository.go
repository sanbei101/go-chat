package ws

import (
	"context"
	"strconv"

	"github.com/sanbei101/go-chat/internal/store"
)

type repository struct {
	queries *store.Queries
}

func NewRepository(queries *store.Queries) Repository {
	return &repository{queries: queries}
}

func (r *repository) CreateRoom(ctx context.Context, room *Room) (*Room, error) {
	createdRoom, err := r.queries.CreateRoom(ctx, room.Name)
	if err != nil {
		return &Room{}, err
	}

	return &Room{
		ID:   strconv.FormatInt(createdRoom.ID, 10),
		Name: createdRoom.Name,
	}, nil
}

func (r *repository) FetchRooms() ([]*Room, error) {
	rows, err := r.queries.ListRooms(context.Background())
	if err != nil {
		return nil, err
	}

	var rooms []*Room
	for _, row := range rows {
		rooms = append(rooms, &Room{
			ID:   strconv.FormatInt(row.ID, 10),
			Name: row.Name,
		})
	}

	return rooms, nil
}

// JoinRoom adds a new entry to room_member table
// if user already exists update last_online time
func (r *repository) JoinRoom(ctx context.Context, client *Client) error {
	roomID, err := strconv.ParseInt(client.RoomID, 10, 64)
	if err != nil {
		return err
	}
	userID, err := strconv.ParseInt(client.ID, 10, 64)
	if err != nil {
		return err
	}

	params := store.RoomMemberExistsParams{
		RoomID: roomID,
		UserID: userID,
	}
	exists, err := r.queries.RoomMemberExists(ctx, params)
	if err != nil {
		return err
	}

	if exists {
		return r.queries.TouchRoomMember(ctx, store.TouchRoomMemberParams{
			RoomID: roomID,
			UserID: userID,
		})
	}

	return r.queries.CreateRoomMember(ctx, store.CreateRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
}

// WriteMessage adds a new message to the room_message table
// It is called asynchronously with websocket messages
func (r *repository) WriteMessage(ctx context.Context, msg *Message) error {
	roomID, err := strconv.ParseInt(msg.RoomID, 10, 64)
	if err != nil {
		return err
	}
	userID, err := strconv.ParseInt(msg.UserID, 10, 64)
	if err != nil {
		return err
	}

	return r.queries.CreateRoomMessage(ctx, store.CreateRoomMessageParams{
		RoomID:  roomID,
		UserID:  userID,
		Message: msg.Content,
	})
}

// FetchMessages retrieves messages for a specific room
// It is called when a user joins a room to load previous messages
func (r *repository) FetchRoomMessages(ctx context.Context, roomID string) ([]*Message, error) {
	parsedRoomID, err := strconv.ParseInt(roomID, 10, 64)
	if err != nil {
		return nil, err
	}

	rows, err := r.queries.ListRecentRoomMessages(ctx, parsedRoomID)
	if err != nil {
		return nil, err
	}

	var messages []*Message
	for _, row := range rows {
		messages = append(messages, &Message{
			RoomID:   strconv.FormatInt(row.RoomID, 10),
			UserID:   strconv.FormatInt(row.UserID, 10),
			Username: row.Username,
			Content:  row.Message,
		})
	}
	return messages, nil
}
