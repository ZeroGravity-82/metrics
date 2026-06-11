package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/httpserver/handler"
	"zerogravity-82/metrics/internal/model"
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

// AuditPublisher публикует события аудита об успешных обновлениях метрик.
type AuditPublisher interface {
	PublishLog(ctx context.Context, now time.Time, ip string, models ...model.Metrics)
}

// HTTPServer - основной API-сервер сервиса метрик.
//
// Он запускает роутер, собранный handler.NewMetricRouter, на указанном адресе.
type HTTPServer struct {
	addr           string
	tlsConfig      *tls.Config
	storage        Storage
	auditPublisher AuditPublisher
	signatureKey   string
	cryptoKeyPath  string
	trustedSubnet  string
	logger         zerolog.Logger
}

// NewHTTPServer создает новый HTTPServer.
func NewHTTPServer(
	addr string,
	tlsConfig *tls.Config,
	storage Storage,
	auditPublisher AuditPublisher,
	signatureKey string,
	cryptoKeyPath string,
	trustedSubnet string,
	logger zerolog.Logger,
) *HTTPServer {
	return &HTTPServer{
		addr:           addr,
		tlsConfig:      tlsConfig,
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
	router, err := handler.NewMetricRouter(
		s.storage,
		s.auditPublisher,
		s.signatureKey,
		s.cryptoKeyPath,
		s.trustedSubnet,
		s.logger,
	)
	if err != nil {
		return fmt.Errorf("failed to build metric router: %w", err)
	}
	srv := http.Server{
		Addr:              s.addr,
		TLSConfig:         s.tlsConfig,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info().Str("address", s.addr).Msg("starting http server")
		errCh <- srv.ListenAndServeTLS("", "")
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
