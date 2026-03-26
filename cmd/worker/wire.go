//go:build wireinject

package main

import (
	"context"
	"errors"
	"net"

	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/sanbei101/im/internal/db"
	"github.com/sanbei101/im/internal/worker"
	"github.com/sanbei101/im/pkg/config"
	proto "github.com/sanbei101/im/pkg/protocol"
)

func provideConfig(path string) (*config.Config, error) {
	return config.Load(path)
}

func providePGXPool(cfg *config.Config) (*pgxpool.Pool, func(), error) {
	if cfg.Postgres.DSN == "" {
		return nil, nil, errors.New("postgres.dsn is required for worker")
	}

	pool, err := pgxpool.New(context.Background(), cfg.Postgres.DSN)
	if err != nil {
		return nil, nil, err
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, nil, err
	}
	return pool, pool.Close, nil
}

func provideQueries(pool *pgxpool.Pool) *db.Queries {
	return db.New(pool)
}

func provideGatewayConn(cfg *config.Config) (*grpc.ClientConn, func(), error) {
	conn, err := grpc.NewClient(cfg.Gateway.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return conn, func() { _ = conn.Close() }, nil
}

func provideGatewayServiceClient(conn *grpc.ClientConn) proto.GatewayServiceClient {
	return proto.NewGatewayServiceClient(conn)
}

func provideDeliverer(client proto.GatewayServiceClient) worker.Deliverer {
	return func(ctx context.Context, msg *proto.ChatMessage) error {
		_, err := client.DeliverMessage(ctx, &proto.DeliverMessageRequest{Message: msg})
		return err
	}
}

func provideWorker(queries *db.Queries, deliverer worker.Deliverer) *worker.Worker {
	return worker.New(worker.NewPersistHandler(queries), deliverer)
}

func provideGRPCListener(cfg *config.Config) (net.Listener, error) {
	return net.Listen("tcp", cfg.Worker.GRPCAddr)
}

func provideWorkerGRPCServer(w *worker.Worker) *WorkerGRPCServer {
	return &WorkerGRPCServer{worker: w}
}

func provideGRPCServer(svc *WorkerGRPCServer) *grpc.Server {
	server := grpc.NewServer()
	proto.RegisterWorkerServiceServer(server, svc)
	return server
}

func provideApp(cfg *config.Config, w *worker.Worker, grpcServer *grpc.Server, grpcListener net.Listener) *App {
	return &App{
		Config:       cfg,
		Worker:       w,
		GRPCServer:   grpcServer,
		GRPCListener: grpcListener,
	}
}

func initializeWorkerApp(path string) (*App, func(), error) {
	wire.Build(
		provideConfig,
		providePGXPool,
		provideQueries,
		provideGatewayConn,
		provideGatewayServiceClient,
		provideDeliverer,
		provideWorker,
		provideGRPCListener,
		provideWorkerGRPCServer,
		provideGRPCServer,
		provideApp,
	)
	return nil, nil, nil
}
