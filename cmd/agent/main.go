// Команда agent запускает агент сбора метрик.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/agent"
	"zerogravity-82/metrics/internal/buildinfo"
	"zerogravity-82/metrics/internal/config"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	buildinfo.Print(buildVersion, buildDate, buildCommit)
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	cfg, err := config.GetAgentConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Config error")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	agent.Run(ctx, cfg, logger)
}
