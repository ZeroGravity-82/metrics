package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"zerogravity-82/metrics/internal/grpcserver/service"
	"zerogravity-82/metrics/internal/model"
	pb "zerogravity-82/metrics/internal/proto"
	"zerogravity-82/metrics/internal/service/audit"
)

const (
	shutdownTimeout = 10 * time.Second
)

// Storage абстрагирует хранилище метрик.
type Storage interface {
	UpdateMetrics(ctx context.Context, metrics []model.Metrics) error
}

// GRPCServer - дополнительный API-сервер сервиса метрик.
type GRPCServer struct {
	addr           string
	storage        Storage
	auditPublisher *audit.AsyncPublisher
	trustedSubnet  string
	logger         zerolog.Logger
}

// NewGRPCServer создает новый GRPCServer.
func NewGRPCServer(
	addr string,
	storage Storage,
	auditPublisher *audit.AsyncPublisher,
	trustedSubnet string,
	logger zerolog.Logger,
) *GRPCServer {
	return &GRPCServer{
		addr:           addr,
		storage:        storage,
		auditPublisher: auditPublisher,
		trustedSubnet:  trustedSubnet,
		logger:         logger,
	}
}

func withTrustedSubnet(trustedSubnetStr string, logger zerolog.Logger) grpc.ServerOption {
	const realIpMetadataKey = "x-real-ip"
	var (
		trustedSubnet *net.IPNet
		err           error
	)
	if trustedSubnetStr != "" {
		_, trustedSubnet, err = net.ParseCIDR(trustedSubnetStr)
		if err != nil {
			logger.Error().Err(err).Str("subnet", trustedSubnetStr).Msg("invalid trusted subnet")
		}
	}

	return grpc.UnaryInterceptor(func(ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		if trustedSubnetStr == "" {
			return handler(ctx, req)
		}
		if trustedSubnet == nil {
			return nil, status.Error(codes.Internal, "Internal server error")
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "metadata is not provided")
		}
		values := md.Get(realIpMetadataKey)
		if len(values) == 0 {
			return nil, status.Errorf(codes.PermissionDenied, "metadata value %s is not provided", realIpMetadataKey)
		}
		ip := net.ParseIP(values[0])
		if ip == nil {
			return nil, status.Errorf(codes.PermissionDenied, "invalid metadata value %s format", realIpMetadataKey)
		}
		if !trustedSubnet.Contains(ip) {
			return nil, status.Error(codes.PermissionDenied, "your IP is not in the trusted subnet")
		}
		return handler(ctx, req)
	})
}

// Run запускает GRPC-сервер и блокируется, пока не отменен контекст или сервер не остановится с ошибкой.
func (s *GRPCServer) Run(ctx context.Context) error {
	listen, err := net.Listen("tcp", s.addr)
	if err != nil {
		s.logger.Error().Err(err).Msg("grpc server failed with error")
		return fmt.Errorf("grpc server error: %w", err)
	}

	srv := grpc.NewServer(withTrustedSubnet(s.trustedSubnet, s.logger))
	pb.RegisterMetricsServer(srv, service.NewMetricsService(s.storage, s.auditPublisher, s.trustedSubnet, s.logger))

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info().Str("address", s.addr).Msg("starting grpc server")
		errCh <- srv.Serve(listen)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		err := s.shutdown(shutdownCtx, srv)
		if err == nil {
			s.logger.Info().Msg("grpc server stopped with graceful shutdown")
			return nil
		}
		s.logger.Error().Err(err).Msg("grpc server stopped with error")
		return fmt.Errorf("grpc server stopped with error: %w", err)
	case err := <-errCh:
		if err == nil || errors.Is(err, grpc.ErrServerStopped) {
			s.logger.Info().Msg("grpc server stopped")
			return nil
		}
		s.logger.Error().Err(err).Msg("grpc server failed with error")
		return fmt.Errorf("grpc server error: %w", err)
	}
}

// shutdown выполняет graceful shutdown gRPC-сервера и принудительно останавливает его,
// если переданный контекст завершился раньше, чем GracefulStop.
func (s *GRPCServer) shutdown(ctx context.Context, srv *grpc.Server) error {
	doneCh := make(chan struct{})

	go func() {
		srv.GracefulStop()
		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		srv.Stop()
		<-doneCh
		return fmt.Errorf("graceful shutdown timeout: %w", ctx.Err())
	case <-doneCh:
		return nil
	}
}
