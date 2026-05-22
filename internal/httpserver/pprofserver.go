package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	// #nosec G108 -- PprofServer.addr is empty by default
	_ "net/http/pprof" // register pprof handlers
	"time"

	"zerogravity-82/metrics/internal/httpserver/handler"

	"github.com/rs/zerolog"
)

// PprofServer - вспомогательный сервер для профилирования.
//
// Пустой адрес отключает запуск pprof-сервера.
type PprofServer struct {
	addr   string
	logger zerolog.Logger
}

// NewPprofServer создает PprofServer для заданного адреса.
func NewPprofServer(addr string, logger zerolog.Logger) *PprofServer {
	return &PprofServer{addr: addr, logger: logger}
}

// Run запускает pprof-сервер и блокируется, пока не отменен контекст или сервер не остановится с ошибкой.
//
// Если адрес пустой, метод ничего не делает и возвращает nil.
func (s *PprofServer) Run(ctx context.Context) error {
	if s.addr == "" {
		return nil
	}
	srv := http.Server{
		Addr:              s.addr,
		Handler:           handler.PprofRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info().Str("address", s.addr).Msg("starting pprof server")
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		err := srv.Shutdown(shutdownCtx)
		if err == nil {
			s.logger.Info().Msg("pprof server stopped with graceful shutdown")
			return nil
		}
		s.logger.Error().Err(err).Msg("pprof server stopped with error")
		return fmt.Errorf("pprof server stopped with error: %w", err)
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			s.logger.Info().Msg("pprof server closed")
			return nil
		}
		s.logger.Error().Err(err).Msg("pprof server failed with error")
		return fmt.Errorf("pprof server error: %w", err)
	}
}
