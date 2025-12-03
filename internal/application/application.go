package application

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/handler"
	"zerogravity-82/metrics/internal/service"
)

type Application struct {
	Logger zerolog.Logger
	Cfg    config.ServerConfig
	FS     *service.FileStorage
	DB     *sql.DB
}

func NewApplication() *Application {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	cfg, err := config.GetServerConfig()
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Config error")
	}

	fs, err := service.NewFileStorage(cfg)
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Storage error")
	}

	db, err := sql.Open("pgx", cfg.DatabaseDSN)
	fmt.Println(cfg.DatabaseDSN)
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Database error")
	}

	return &Application{
		Logger: logger,
		Cfg:    cfg,
		FS:     fs,
		DB:     db,
	}
}

func (app *Application) Run() {
	app.Logger.Info().Str("address", app.Cfg.ServerAddr).Msg("Server started")
	err := http.ListenAndServe(app.Cfg.ServerAddr, handler.MetricRouter(app.FS, app.Logger, app.DB))
	if !errors.Is(err, http.ErrServerClosed) {
		app.Logger.Fatal().Msg(err.Error())
	}
}

func (app *Application) Close() {
	if err := app.DB.Close(); err != nil {
		app.Logger.Error().Msg(err.Error())
	}
	if err := app.FS.Close(); err != nil {
		app.Logger.Error().Msg(err.Error())
	}
}
