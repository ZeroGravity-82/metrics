package main

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/handler"
	"zerogravity-82/metrics/internal/service"
)

func main() {
	cfg, err := config.GetServerConfig()
	if err != nil {
		log.Fatal().Str("error", err.Error()).Msg("Config error")
	}
	ms, err := service.NewMemStorage(cfg)
	if err != nil {
		log.Fatal().Str("error", err.Error()).Msg("MemStorage error")
	}
	log.Info().Str("address", cfg.ServerAddr).Msg("Server started")
	err = http.ListenAndServe(cfg.ServerAddr, handler.MetricRouter(ms, cfg))
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Msg(err.Error())
	}
}
