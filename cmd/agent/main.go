package main

import (
	"os"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/agent"
	"zerogravity-82/metrics/internal/config"
)

func main() {
	// Logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Configuration
	cfg, err := config.GetAgentConfig()
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Config error")
	}

	// Agent
	agent.Run(cfg, logger)
}
