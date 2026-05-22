// Команда server запускает HTTP-сервер сервиса метрик.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/application"
	"zerogravity-82/metrics/internal/buildinfo"
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
		logger.Fatal().Err(err).Msg("Service terminated with error")
	}
}

func run(logger zerolog.Logger) error {
	app, err := application.NewApplication(logger)
	if err != nil {
		return fmt.Errorf("initialization error: %w", err)
	}
	defer app.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	if err = app.Run(ctx); err != nil {
		return fmt.Errorf("execution error: %w", err)
	}
	return nil
}
