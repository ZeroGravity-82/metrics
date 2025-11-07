package main

import (
	"github.com/rs/zerolog/log"

	"zerogravity-82/metrics/internal/agent"
	"zerogravity-82/metrics/internal/config"
)

func main() {
	cfg, err := config.GetAgentConfig()
	if err != nil {
		log.Fatal().Str("error", err.Error()).Msg("Config error")
	}
	agent.Run(cfg)
}
