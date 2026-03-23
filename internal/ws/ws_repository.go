package ws

import (
	"context"
	"database/sql"
	"strconv"
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type repository struct {
	db DBTX
}

func NewRepository(db DBTX) Repository {
	return &repository{db: db}
}

func (r *repository) CreateRoom(ctx context.Context, room *Room) (*Room, error) {
	var lastInsertID int
	query := `INSERT INTO room (name) VALUES ($1) RETURNING id`
	err := r.db.QueryRowContext(ctx, query, room.Name).Scan(&lastInsertID)

	if err != nil {
		return &Room{}, err
	}

	room.ID = strconv.Itoa(lastInsertID)
	return room, nil
}

func (r *repository) FetchRooms() ([]*Room, error) {
	query := `SELECT id, name FROM room`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*Room
	for rows.Next() {
		var room Room
		if err := rows.Scan(&room.ID, &room.Name); err != nil {
			return nil, err
		}
		rooms = append(rooms, &room)
	}

	return rooms, nil
}

// JoinRoom adds a new entry to room_member table
// if user already exists update last_online time
func (r *repository) JoinRoom(ctx context.Context, client *Client) error {
	query := `SELECT FROM room_member WHERE room_id = $1 AND user_id = $2`
	err := r.db.QueryRowContext(ctx, query, client.RoomID, client.ID).Scan()

	switch err {
	case sql.ErrNoRows:
		query = `INSERT INTO room_member (room_id, user_id) VALUES ($1, $2)`
		_, err = r.db.ExecContext(ctx, query, client.RoomID, client.ID)
	case nil:
		query = `UPDATE room_member SET last_online = NOW() WHERE room_id = $1 and user_id = $2`
		_, err = r.db.ExecContext(ctx, query, client.RoomID, client.ID)
	default:
		return err
	}

	return nil
}

// WriteMessage adds a new message to the room_message table
// It is called asynchronously with websocket messages
func (r *repository) WriteMessage(ctx context.Context, msg *Message) error {
	query := `INSERT INTO room_message (room_id, user_id, message) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, msg.RoomID, msg.UserID, msg.Content)
	if err != nil {
		return err
	}

	return nil
}

// FetchMessages retrieves messages for a specific room
// It is called when a user joins a room to load previous messages
func (r *repository) FetchRoomMessages(ctx context.Context, roomID string) ([]*Message, error) {
	query := `
        SELECT rm.user_id, u.username, rm.message
        FROM room_message rm
        JOIN users u ON rm.user_id = u.id
        WHERE rm.room_id = $1 AND
    	rm.created_at >= NOW() - INTERVAL '1 hour'
        ORDER BY rm.created_at ASC
    `
	rows, err := r.db.QueryContext(ctx, query, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.UserID, &msg.Username, &msg.Content); err != nil {
			return nil, err
		}
		msg.RoomID = roomID
		messages = append(messages, &msg)
	}
	return messages, nil
}
