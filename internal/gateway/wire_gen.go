// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package main

import (
	"github.com/google/wire"
	"go-chat/config"
	"go-chat/internal/gateway/internal/consume"
	"go-chat/internal/gateway/internal/event"
	"go-chat/internal/gateway/internal/event/chat"
	"go-chat/internal/gateway/internal/handler"
	"go-chat/internal/gateway/internal/process"
	"go-chat/internal/gateway/internal/router"
	"go-chat/internal/provider"
	"go-chat/internal/repository/cache"
	"go-chat/internal/repository/repo"
	"go-chat/internal/service"
)

// Injectors from wire.go:

func Initialize(conf *config.Config) *AppProvider {
	client := provider.NewRedisClient(conf)
	serverStorage := cache.NewSidStorage(client)
	clientStorage := cache.NewClientStorage(client, conf, serverStorage)
	roomStorage := cache.NewRoomStorage(client)
	db := provider.NewMySQLClient(conf)
	baseService := service.NewBaseService(db, client)
	relation := cache.NewRelation(client)
	groupMember := repo.NewGroupMember(db, relation)
	groupMemberService := service.NewGroupMemberService(baseService, groupMember)
	chatHandler := chat.NewHandler(client, groupMemberService)
	chatEvent := event.NewChatEvent(client, conf, roomStorage, groupMemberService, chatHandler)
	chatChannel := handler.NewChatChannel(clientStorage, chatEvent)
	exampleEvent := event.NewExampleEvent()
	exampleChannel := handler.NewExampleChannel(clientStorage, exampleEvent)
	handlerHandler := &handler.Handler{
		Chat:    chatChannel,
		Example: exampleChannel,
		Config:  conf,
	}
	tokenSessionStorage := cache.NewTokenSessionStorage(client)
	engine := router.NewRouter(conf, handlerHandler, tokenSessionStorage)
	websocketServer := provider.NewWebsocketServer(conf, engine)
	healthSubscribe := process.NewHealthSubscribe(conf, serverStorage)
	talkVote := cache.NewTalkVote(client)
	talkRecordsVote := repo.NewTalkRecordsVote(db, talkVote)
	talkRecords := repo.NewTalkRecords(db)
	talkRecordsService := service.NewTalkRecordsService(baseService, talkVote, talkRecordsVote, groupMember, talkRecords)
	contactRemark := cache.NewContactRemark(client)
	contact := repo.NewContact(db, contactRemark, relation)
	contactService := service.NewContactService(baseService, contact)
	chatSubscribe := consume.NewChatSubscribe(conf, clientStorage, roomStorage, talkRecordsService, contactService)
	exampleSubscribe := consume.NewExampleSubscribe()
	messageSubscribe := process.NewMessageSubscribe(conf, client, chatSubscribe, exampleSubscribe)
	subServers := &process.SubServers{
		HealthSubscribe:  healthSubscribe,
		MessageSubscribe: messageSubscribe,
	}
	server := process.NewServer(subServers)
	appProvider := &AppProvider{
		Config:    conf,
		Server:    websocketServer,
		Coroutine: server,
		Handler:   handlerHandler,
	}
	return appProvider
}

// wire.go:

var providerSet = wire.NewSet(provider.NewMySQLClient, provider.NewRedisClient, provider.NewWebsocketServer, router.NewRouter, wire.Struct(new(process.SubServers), "*"), process.NewServer, process.NewHealthSubscribe, process.NewMessageSubscribe, consume.NewChatSubscribe, consume.NewExampleSubscribe, cache.NewTokenSessionStorage, cache.NewSidStorage, cache.NewRedisLock, cache.NewClientStorage, cache.NewRoomStorage, cache.NewTalkVote, cache.NewRelation, cache.NewContactRemark, cache.NewSequence, repo.NewTalkRecords, repo.NewTalkRecordsVote, repo.NewGroupMember, repo.NewContact, chat.NewHandler, event.NewChatEvent, event.NewExampleEvent, service.NewBaseService, service.NewTalkRecordsService, service.NewGroupMemberService, service.NewContactService, handler.NewChatChannel, handler.NewExampleChannel, wire.Struct(new(handler.Handler), "*"), wire.Struct(new(AppProvider), "*"))
