package application

import (
	"errors"
	"net/http"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/handler"
	"zerogravity-82/metrics/internal/repository"
)

type Application struct {
	Logger  zerolog.Logger
	Cfg     config.ServerConfig
	Storage handler.Storage
}

func NewApplication() *Application {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	cfg, err := config.GetServerConfig()
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Config error")
	}

	var storage handler.Storage
	if cfg.DatabaseDSN != "" {
		db, err := sqlx.Connect("pgx", cfg.DatabaseDSN)
		if err != nil {
			logger.Fatal().Str("error", err.Error()).Msg("Failed to connect to the database")
		}
		applyMigrations(db, logger)
		storage = repository.NewDBStorage(db)
	} else if cfg.FileStoragePath != "" {
		storage, err = repository.NewFileStorage(cfg)
		if err != nil {
			logger.Fatal().Str("error", err.Error()).Msg("Storage error")
		}
	} else {
		storage = repository.NewMemStorage()
	}
	return &Application{
		Logger:  logger,
		Cfg:     cfg,
		Storage: storage,
	}
}

func applyMigrations(db *sqlx.DB, logger zerolog.Logger) {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Failed to initialize database driver")
	}
	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		logger.Fatal().Str("error", err.Error()).Msg("Failed to initialize migrations")
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		logger.Fatal().Str("error", err.Error()).Msg("Failed to apply migrations")
	}
}

func (app *Application) Run() {
	app.Logger.Info().Str("address", app.Cfg.ServerAddr).Msg("Server started")
	err := http.ListenAndServe(app.Cfg.ServerAddr, handler.MetricRouter(app.Storage, app.Logger))
	if !errors.Is(err, http.ErrServerClosed) {
		app.Logger.Fatal().Msg(err.Error())
	}
}

func (app *Application) Close() {
	if err := app.Storage.Close(); err != nil {
		app.Logger.Error().Msg(err.Error())
	}
}
