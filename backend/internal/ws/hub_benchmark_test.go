package ws

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sanbei101/go-chat/db"
	"github.com/sanbei101/go-chat/internal/store"
)

type trackingRepository struct {
	inner Repository
	wg    sync.WaitGroup
}

func (r *trackingRepository) CreateRoom(ctx context.Context, room *Room) (*Room, error) {
	return r.inner.CreateRoom(ctx, room)
}

func (r *trackingRepository) FetchRooms() ([]*Room, error) {
	return r.inner.FetchRooms()
}

func (r *trackingRepository) JoinRoom(ctx context.Context, client *Client) error {
	return r.inner.JoinRoom(ctx, client)
}

func (r *trackingRepository) WriteMessage(ctx context.Context, msg *Message) error {
	defer r.wg.Done()
	return r.inner.WriteMessage(ctx, msg)
}

func (r *trackingRepository) FetchRoomMessages(ctx context.Context, roomID string) ([]*Message, error) {
	return r.inner.FetchRoomMessages(ctx, roomID)
}

func (r *trackingRepository) Wait() {
	r.wg.Wait()
}

func mustPrepareBenchmarkDatabase(b *testing.B, dbtx store.DBTX, queries *store.Queries, clientCount int) string {
	b.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id bigserial PRIMARY KEY,
			username varchar NOT NULL,
			email varchar NOT NULL UNIQUE,
			password varchar NOT NULL,
			create_date timestamp NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS room (
			id bigserial PRIMARY KEY,
			name varchar NOT NULL,
			create_date timestamp NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS room_member (
			id bigserial PRIMARY KEY,
			room_id bigint NOT NULL REFERENCES room(id),
			user_id bigint NOT NULL REFERENCES users(id),
			join_date timestamp NOT NULL DEFAULT now(),
			last_online timestamp NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS room_message (
			id bigserial PRIMARY KEY,
			room_id bigint NOT NULL REFERENCES room(id) ON DELETE CASCADE,
			user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			message text NOT NULL,
			created_at timestamp NOT NULL DEFAULT now()
		)`,
		`TRUNCATE TABLE room_message, room_member, room, users RESTART IDENTITY CASCADE`,
	}

	for _, stmt := range statements {
		if _, err := dbtx.Exec(ctx, stmt); err != nil {
			b.Fatalf("prepare benchmark database: %v", err)
		}
	}

	room, err := queries.CreateRoom(ctx, fmt.Sprintf("bench-room-%d", clientCount))
	if err != nil {
		b.Fatalf("create benchmark room: %v", err)
	}

	for i := range clientCount {
		user, err := queries.CreateUser(ctx, store.CreateUserParams{
			Username: fmt.Sprintf("bench-user-%d", i),
			Email:    fmt.Sprintf("bench-user-%d@example.com", i),
			Password: "benchmark-password",
		})
		if err != nil {
			b.Fatalf("create benchmark user %d: %v", i, err)
		}

		if err := queries.CreateRoomMember(ctx, store.CreateRoomMemberParams{
			RoomID: room.ID,
			UserID: user.ID,
		}); err != nil {
			b.Fatalf("create benchmark room member %d: %v", i, err)
		}
	}

	return strconv.FormatInt(room.ID, 10)
}

func newBenchmarkHub(b *testing.B, clientCount int) (*Hub, *trackingRepository, string) {
	b.Helper()

	if os.Getenv("POSTGRES_URL") == "" {
		b.Skip("POSTGRES_URL is not set")
	}

	dbConn, err := db.NewDatabase()
	if err != nil {
		b.Fatalf("connect benchmark database: %v", err)
	}
	b.Cleanup(dbConn.Close)

	queries := store.New(dbConn.GetDB())
	roomID := mustPrepareBenchmarkDatabase(b, dbConn.GetDB(), queries, clientCount)

	repo := &trackingRepository{
		inner: NewRepository(queries),
	}
	hub := NewHub(repo)
	hub.Rooms[roomID] = &Room{
		ID:      roomID,
		Name:    "bench-room",
		Clients: make(map[string]*Client, clientCount),
	}

	for i := range clientCount {
		clientID := strconv.Itoa(i + 1)
		hub.Rooms[roomID].Clients[clientID] = &Client{
			ID:       clientID,
			RoomID:   roomID,
			Username: "user-" + clientID,
			Message:  make(chan *Message, 1),
		}
	}

	return hub, repo, roomID
}

func BenchmarkHubBroadcast(b *testing.B) {
	for _, clientCount := range []int{1, 10, 100, 500} {
		b.Run("clients_"+strconv.Itoa(clientCount), func(b *testing.B) {
			hub, repo, roomID := newBenchmarkHub(b, clientCount)
			msg := &Message{
				Content:  "benchmark message",
				RoomID:   roomID,
				Username: "sender",
				UserID:   "1",
			}
			for b.Loop() {
				repo.wg.Add(1)
				hub.broadcastMessage(msg)

				for _, cl := range hub.Rooms[msg.RoomID].Clients {
					<-cl.Message
				}
			}
			repo.Wait()
		})
	}
}
