package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/sanbei101/go-chat/config"
	"github.com/sanbei101/go-chat/db"
	"github.com/sanbei101/go-chat/internal/store"
	"github.com/sanbei101/go-chat/internal/user"
	"github.com/sanbei101/go-chat/internal/ws"
	"github.com/sanbei101/go-chat/router"
)

func main() {
	dbConn, err := db.NewDatabase()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer dbConn.Close()

	queries := store.New(dbConn.GetDB())

	userRep := user.NewRepository(queries)
	userSvc := user.NewService(userRep)
	userHndlr := user.NewHandler(userSvc)

	wsRep := ws.NewRepository(queries)
	hub := ws.NewHub(wsRep)
	wsSvc := ws.NewService(wsRep, hub)
	wsHndlr := ws.NewHandler(wsSvc)
	go hub.Run()

	conf := config.LoadConfig()

	r := router.Init(userHndlr, wsHndlr)
	port, _ := strconv.Atoi(conf.ServerPort)
	addr := fmt.Sprintf("%s:%d", conf.ServerHost, port)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
