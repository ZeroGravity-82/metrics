package main

import (
	"errors"
	"net/http"
	"os"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/handler"
	"zerogravity-82/metrics/internal/service"
)

func main() {
	// Logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Configuration
	cfg, err := config.GetServerConfig()
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Config error")
	}

	// Storage
	ms, err := service.NewMemStorage(cfg)
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("MemStorage error")
	}

	// HTTP server
	logger.Info().Str("address", cfg.ServerAddr).Msg("Server started")
	err = http.ListenAndServe(cfg.ServerAddr, handler.MetricRouter(ms, cfg, logger))
	if !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal().Msg(err.Error())
	}
}
