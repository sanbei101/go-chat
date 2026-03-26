//go:build wireinject

package main

import (
	"context"
	"net"
	"net/http"

	"github.com/google/wire"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/sanbei101/im/internal/gateway"
	"github.com/sanbei101/im/pkg/config"
	proto "github.com/sanbei101/im/pkg/protocol"
)

func provideConfig(path string) (*config.Config, error) {
	return config.Load(path)
}

func provideWorkerConn(cfg *config.Config) (*grpc.ClientConn, func(), error) {
	conn, err := grpc.NewClient(cfg.Worker.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return conn, func() { _ = conn.Close() }, nil
}

func provideWorkerServiceClient(conn *grpc.ClientConn) proto.WorkerServiceClient {
	return proto.NewWorkerServiceClient(conn)
}

func provideGatewayPublisher(client proto.WorkerServiceClient) gateway.Publisher {
	return gateway.PublisherFunc(func(ctx context.Context, envelope *gateway.InboundEnvelope) error {
		_, err := client.SendMessage(ctx, &proto.SendMessageRequest{Message: envelope.Message})
		return err
	})
}

func provideGatewayAuthFunc() gateway.AuthFunc {
	return authFromRequest
}

func provideGateway(_ *config.Config, auth gateway.AuthFunc, publisher gateway.Publisher) *gateway.Gateway {
	return gateway.New(auth, publisher)
}

func provideHTTPHandler(cfg *config.Config, gw *gateway.Gateway) http.Handler {
	return buildMux(cfg, gw)
}

func provideHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Gateway.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Gateway.ReadHeaderTimeout,
	}
}

func provideGRPCListener(cfg *config.Config) (net.Listener, error) {
	return net.Listen("tcp", cfg.Gateway.GRPCAddr)
}

func provideGatewayGRPCServer(gw *gateway.Gateway) *GatewayGRPCServer {
	return &GatewayGRPCServer{gw: gw}
}

func provideGRPCServer(svc *GatewayGRPCServer) *grpc.Server {
	server := grpc.NewServer()
	proto.RegisterGatewayServiceServer(server, svc)
	return server
}

func provideApp(cfg *config.Config, gw *gateway.Gateway, httpServer *http.Server, grpcServer *grpc.Server, grpcListener net.Listener) *App {
	return &App{
		Config:       cfg,
		Gateway:      gw,
		HTTPServer:   httpServer,
		GRPCServer:   grpcServer,
		GRPCListener: grpcListener,
	}
}

func initializeGatewayApp(path string) (*App, func(), error) {
	wire.Build(
		provideConfig,
		provideWorkerConn,
		provideWorkerServiceClient,
		provideGatewayAuthFunc,
		provideGatewayPublisher,
		provideGateway,
		provideHTTPHandler,
		provideHTTPServer,
		provideGRPCListener,
		provideGatewayGRPCServer,
		provideGRPCServer,
		provideApp,
	)
	return nil, nil, nil
}
