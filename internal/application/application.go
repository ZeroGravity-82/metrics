package application

import (
	"errors"
	"fmt"
	"net/http"

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
	logger  zerolog.Logger
	cfg     config.ServerConfig
	storage handler.Storage
}

func NewApplication(logger zerolog.Logger) (*Application, error) {
	cfg, err := config.GetServerConfig()
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	var storage handler.Storage
	if cfg.DatabaseDSN != "" {
		db, err := sqlx.Connect("pgx", cfg.DatabaseDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to the database: %w", err)
		}
		err = applyMigrations(db)
		if err != nil {
			return nil, fmt.Errorf("migrations error: %w", err)
		}
		storage = repository.NewDBStorage(db)
	} else if cfg.FileStoragePath != "" {
		storage, err = repository.NewFileStorage(cfg)
		if err != nil {
			return nil, fmt.Errorf("storage error: %w", err)
		}
	} else {
		storage = repository.NewMemStorage()
	}
	return &Application{
		logger:  logger,
		cfg:     cfg,
		storage: storage,
	}, nil
}

func applyMigrations(db *sqlx.DB) error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to initialize database driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	return nil
}

func (app *Application) Run() error {
	app.logger.Info().Str("address", app.cfg.ServerAddr).Msg("Server started")
	err := http.ListenAndServe(app.cfg.ServerAddr, handler.MetricRouter(app.storage, app.cfg.Key, app.logger))
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP server error: %w", err)
	}
	return nil
}

func (app *Application) Close() {
	if err := app.storage.Close(); err != nil {
		app.logger.Error().Msg(err.Error())
	}
}
