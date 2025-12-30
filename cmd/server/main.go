package main

import (
	"os"

	"zerogravity-82/metrics/internal/application"

	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	app, err := application.NewApplication(logger)
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Initialization error")
	}
	defer app.Close()

	err = app.Run()
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Execution error")
	}
}
