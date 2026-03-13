package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/application"
)

func main() {
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

	if err = app.Run(context.Background()); err != nil {
		return fmt.Errorf("execution error: %w", err)
	}
	return nil
}
