package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"zerogravity-82/metrics/internal/httpserver/handler"

	"github.com/rs/zerolog"
)

// PprofServer runs Go's net/http/pprof handlers on a dedicated address.
// Address can be empty to disable the server.
type PprofServer struct {
	addr   string
	logger zerolog.Logger
}

func NewPprofServer(addr string, logger zerolog.Logger) *PprofServer {
	return &PprofServer{addr: addr, logger: logger}
}

func (s *PprofServer) Run(_ context.Context) error {
	if s.addr == "" {
		return nil
	}
	s.logger.Info().Str("address", s.addr).Msg("pprof server started")
	err := http.ListenAndServe(s.addr, handler.PprofRouter())
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("pprof server error: %w", err)
	}
	return nil
}
