package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/httpserver/handler"
	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/service/audit"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Storage абстрагирует хранилище метрик.
type Storage interface {
	UpdateMetric(ctx context.Context, m model.Metrics) error
	UpdateMetrics(ctx context.Context, metrics []model.Metrics) error
	GetMetric(ctx context.Context, mType, mName string) (model.Metrics, error)
	GetAll(ctx context.Context) (map[string]model.Metrics, error)
	Ping(ctx context.Context) error
}

// HTTPServer - основной API-сервер сервиса метрик.
//
// Он запускает роутер, собранный handler.MetricRouter, на указанном адресе.
type HTTPServer struct {
	addr           string
	storage        Storage
	auditPublisher *audit.AsyncPublisher
	signatureKey   string
	cryptoKeyPath  string
	trustedSubnet  string
	logger         zerolog.Logger
}

// NewHTTPServer создает новый HTTPServer.
func NewHTTPServer(
	addr string,
	storage Storage,
	auditPublisher *audit.AsyncPublisher,
	signatureKey string,
	cryptoKeyPath string,
	trustedSubnet string,
	logger zerolog.Logger,
) *HTTPServer {
	return &HTTPServer{
		addr:           addr,
		storage:        storage,
		auditPublisher: auditPublisher,
		signatureKey:   signatureKey,
		cryptoKeyPath:  cryptoKeyPath,
		trustedSubnet:  trustedSubnet,
		logger:         logger,
	}
}

// Run запускает HTTP-сервер и блокируется, пока не отменен контекст или сервер не остановится с ошибкой.
func (s *HTTPServer) Run(ctx context.Context) error {
	srv := http.Server{
		Addr: s.addr,
		Handler: handler.MetricRouter(
			s.storage,
			s.auditPublisher,
			s.signatureKey,
			s.cryptoKeyPath,
			s.trustedSubnet,
			s.logger,
		),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info().Str("address", s.addr).Msg("starting http server")
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		err := srv.Shutdown(shutdownCtx)
		if err == nil {
			s.logger.Info().Msg("http server stopped with graceful shutdown")
			return nil
		}
		s.logger.Error().Err(err).Msg("http server stopped with error")
		return fmt.Errorf("http server stopped with error: %w", err)
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			s.logger.Info().Msg("http server closed")
			return nil
		}
		s.logger.Error().Err(err).Msg("http server failed with error")
		return fmt.Errorf("http server error: %w", err)
	}
}
