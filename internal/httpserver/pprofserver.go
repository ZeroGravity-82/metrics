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

// Run запускает pprof-сервер.
//
// Если адрес пустой, метод ничего не делает и возвращает nil.
func (s *PprofServer) Run(_ context.Context) error {
	if s.addr == "" {
		return nil
	}
	s.logger.Info().Str("address", s.addr).Msg("pprof server started")
	srv := http.Server{
		Addr:              s.addr,
		Handler:           handler.PprofRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("pprof server error: %w", err)
	}
	return nil
}
