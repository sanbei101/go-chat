package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sanbei101/go-chat/config"
)

type Database struct {
	db *pgxpool.Pool
}

func NewDatabase() (*Database, error) {
	postgresURL := config.LoadConfig().PostgresUrl

	db, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(context.Background()); err != nil {
		db.Close()
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() {
	d.db.Close()
}

func (d *Database) GetDB() *pgxpool.Pool {
	return d.db
}
