package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/httpserver"
	"zerogravity-82/metrics/internal/httpserver/handler"
	"zerogravity-82/metrics/internal/repository"
	"zerogravity-82/metrics/internal/service/audit"
)

type Application struct {
	logger         zerolog.Logger
	cfg            config.ServerConfig
	storage        handler.Storage
	srv            *httpserver.HTTPServer
	auditPublisher *audit.AsyncPublisher
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
		storage, err = repository.NewFileStorage(cfg.FileStoragePath, cfg.Restore)
		if err != nil {
			return nil, fmt.Errorf("storage error: %w", err)
		}
	} else {
		storage = repository.NewMemStorage()
	}
	publisher := buildAuditPublisher(cfg, logger)
	srv := httpserver.NewHTTPServer(cfg.ServerAddr, storage, publisher, cfg.Key, logger)

	return &Application{
		logger:         logger,
		cfg:            cfg,
		storage:        storage,
		srv:            srv,
		auditPublisher: publisher,
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

func (app *Application) Run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error { return app.srv.Run(app.logger) })
	eg.Go(func() error { app.auditPublisher.Run(ctx); return nil })
	return eg.Wait()
}

func (app *Application) Close() {
	if err := app.storage.Close(); err != nil {
		app.logger.Error().Msg(err.Error())
	}
}

func buildAuditPublisher(cfg config.ServerConfig, logger zerolog.Logger) *audit.AsyncPublisher {
	var observers []audit.Observer
	if cfg.AuditFile != "" {
		observers = append(observers, audit.NewFileObserver(cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		observers = append(observers, audit.NewHTTPObserver(cfg.AuditURL))
	}
	return audit.NewAsyncPublisher(logger, observers...)
}
