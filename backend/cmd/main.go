package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sanbei101/go-chat/config"
	"github.com/sanbei101/go-chat/db"
	"github.com/sanbei101/go-chat/internal/store"
	"github.com/sanbei101/go-chat/internal/user"
	"github.com/sanbei101/go-chat/internal/ws"
	"github.com/sanbei101/go-chat/router"
)

func main() {
	conf := config.LoadConfig()

	dbConn, err := db.NewDatabase()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer dbConn.Close()

	queries := store.New(dbConn.GetDB())

	userRep := user.NewRepository(queries)
	userSvc := user.NewService(userRep)
	userHndlr := user.NewHandler(userSvc)

	redisDB, err := strconv.Atoi(conf.RedisDB)
	if err != nil {
		redisDB = 0
	}
	if conf.RedisAddr == "" {
		conf.RedisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     conf.RedisAddr,
		Password: conf.RedisPass,
		DB:       redisDB,
	})
	defer rdb.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	wsRep := ws.NewRepository(queries)
	hub := ws.NewHub(wsRep, rdb)
	if err := hub.Start(context.Background()); err != nil {
		log.Fatalf("start ws hub failed: %v", err)
	}
	wsSvc := ws.NewService(wsRep, hub)
	wsHndlr := ws.NewHandler(wsSvc)

	r := router.Init(userHndlr, wsHndlr)
	port, _ := strconv.Atoi(conf.ServerPort)
	addr := fmt.Sprintf("%s:%d", conf.ServerHost, port)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
