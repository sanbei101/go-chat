package main

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/phuslu/log"
	"google.golang.org/grpc"

	"github.com/sanbei101/im/internal/worker"
	"github.com/sanbei101/im/pkg/config"
	proto "github.com/sanbei101/im/pkg/protocol"
)

type App struct {
	Config       *config.Config
	Worker       *worker.Service
	GRPCServer   *grpc.Server
	GRPCListener net.Listener
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info().
			Str("grpc_addr", a.Config.Worker.GRPCAddr).
			Msg("worker grpc server started")

		if err := a.GRPCServer.Serve(a.GRPCListener); err != nil {
			errCh <- err
		}
	}()

	go func() {
		<-ctx.Done()
		a.GRPCServer.GracefulStop()
	}()

	select {
	case <-ctx.Done():
		wg.Wait()
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, grpc.ErrServerStopped) {
			wg.Wait()
			return ctx.Err()
		}
		return err
	}
}

type WorkerGRPCServer struct {
	proto.UnimplementedWorkerServiceServer
	worker *worker.Service
}

func (s *WorkerGRPCServer) SendMessage(ctx context.Context, req *proto.SendMessageRequest) (*proto.SendMessageResponse, error) {
	if req == nil || req.Message == nil {
		return &proto.SendMessageResponse{}, nil
	}

	delivery, err := s.worker.Process(ctx, &worker.InboundEnvelope{
		Message: req.Message,
	})
	if err != nil {
		return nil, err
	}
	return &proto.SendMessageResponse{Message: delivery.Message}, nil
}
