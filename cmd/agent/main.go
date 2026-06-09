// Команда agent запускает агент сбора метрик.
package main

import (
	"context"
	"fmt"
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

	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("Agent terminated with error")
	}
}

func run(logger zerolog.Logger) error {
	cfg, err := config.GetAgentConfig()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	if err = agent.Run(ctx, cfg, logger); err != nil {
		return fmt.Errorf("execution error: %w", err)
	}
	return nil
}
