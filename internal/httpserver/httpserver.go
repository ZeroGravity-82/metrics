package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/httpserver/handler"
	"zerogravity-82/metrics/internal/service/audit"
)

type HTTPServer struct {
	addr           string
	storage        handler.Storage
	auditPublisher *audit.AsyncPublisher
	key            string
	logger         zerolog.Logger
}

func NewHTTPServer(
	addr string,
	storage handler.Storage,
	auditPublisher *audit.AsyncPublisher,
	key string,
	logger zerolog.Logger,
) *HTTPServer {
	return &HTTPServer{
		addr:           addr,
		storage:        storage,
		auditPublisher: auditPublisher,
		key:            key,
		logger:         logger,
	}
}

func (s *HTTPServer) Run(_ context.Context) error {
	s.logger.Info().Str("address", s.addr).Msg("http server started")
	err := http.ListenAndServe(
		s.addr,
		handler.MetricRouter(s.storage, s.auditPublisher, s.key, s.logger),
	)
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server error: %w", err)
	}
	return nil
}
