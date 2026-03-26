package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/phuslu/log"
	"google.golang.org/grpc"

	"github.com/sanbei101/im/internal/gateway"
	"github.com/sanbei101/im/pkg/config"
	proto "github.com/sanbei101/im/pkg/protocol"
)

type App struct {
	Config       *config.Config
	Gateway      *gateway.Gateway
	HTTPServer   *http.Server
	GRPCServer   *grpc.Server
	GRPCListener net.Listener
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		log.Info().
			Str("addr", a.Config.Gateway.Addr).
			Str("path", a.Config.Gateway.Path).
			Msg("gateway http server started")

		if err := a.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		defer wg.Done()
		log.Info().
			Str("grpc_addr", a.Config.Gateway.GRPCAddr).
			Msg("gateway grpc server started")

		if err := a.GRPCServer.Serve(a.GRPCListener); err != nil {
			errCh <- err
		}
	}()

	go a.shutdownOnDone(ctx)

	select {
	case <-ctx.Done():
		wg.Wait()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (a *App) shutdownOnDone(ctx context.Context) {
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.Config.Gateway.ShutdownTimeout)
	defer cancel()

	a.Gateway.Close(websocket.StatusGoingAway, "server shutdown")
	a.GRPCServer.GracefulStop()
	if err := a.HTTPServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("shutdown gateway http server failed")
	}
}

type GatewayGRPCServer struct {
	proto.UnimplementedGatewayServiceServer
	gw *gateway.Gateway
}

func (s *GatewayGRPCServer) DeliverMessage(ctx context.Context, req *proto.DeliverMessageRequest) (*proto.DeliverMessageResponse, error) {
	if req == nil || req.Message == nil {
		return &proto.DeliverMessageResponse{}, nil
	}
	if err := s.gw.Push(ctx, req.Message.GetReceiverId(), req.Message); err != nil && !errors.Is(err, gateway.ErrUserNotFound) {
		return nil, err
	}
	return &proto.DeliverMessageResponse{}, nil
}

func buildMux(cfg *config.Config, gw *gateway.Gateway) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(cfg.Gateway.Path, gw)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func authFromRequest(_ context.Context, r *http.Request, token string) (string, error) {
	if userID := strings.TrimSpace(r.Header.Get("X-User-ID")); userID != "" {
		return userID, nil
	}
	if userID := strings.TrimSpace(r.URL.Query().Get("user_id")); userID != "" {
		return userID, nil
	}
	if token != "" {
		return token, nil
	}
	return "", gateway.ErrUnauthorized
}
