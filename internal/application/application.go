// Пакет application собирает и запускает основные компоненты сервиса.
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

// Application связывает конфигурацию, хранилище, аудитора запросов и HTTP-сервера в единый сервис.
//
// Используется в cmd/server для сборки и запуска сервиса.
type Application struct {
	logger         zerolog.Logger
	cfg            config.ServerConfig
	storage        handler.Storage
	httpSrv        *httpserver.HTTPServer
	pprofSrv       *httpserver.PprofServer
	auditPublisher *audit.AsyncPublisher
}

// NewApplication собирает Application с учетом настроек из переменных окружения и флагов.
func NewApplication(logger zerolog.Logger) (*Application, error) {
	cfg, err := config.GetServerConfig()
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	var storage handler.Storage
	switch true {
	case cfg.DatabaseDSN != "":
		var db *sqlx.DB
		db, err = sqlx.Connect("pgx", cfg.DatabaseDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to the database: %w", err)
		}
		err = applyMigrations(db)
		if err != nil {
			return nil, fmt.Errorf("migrations error: %w", err)
		}
		storage = repository.NewDBStorage(db)
	case cfg.FileStoragePath != "":
		storage, err = repository.NewFileStorage(cfg.FileStoragePath, cfg.Restore)
		if err != nil {
			return nil, fmt.Errorf("storage error: %w", err)
		}
	default:
		storage = repository.NewMemStorage()
	}
	publisher, err := buildAuditPublisher(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("audit publisher error: %w", err)
	}
	httpSrv := httpserver.NewHTTPServer(cfg.ServerAddr, storage, publisher, cfg.SignatureKey, cfg.CryptoKeyPath, logger)
	pprofSrv := httpserver.NewPprofServer(cfg.PprofAddr, logger)

	return &Application{
		logger:         logger,
		cfg:            cfg,
		storage:        storage,
		httpSrv:        httpSrv,
		pprofSrv:       pprofSrv,
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

// Run запускает основные подсистемы сервиса и ждет их завершения.
func (app *Application) Run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error { return app.httpSrv.Run(ctx) })
	eg.Go(func() error { return app.pprofSrv.Run(ctx) })
	eg.Go(func() error { app.auditPublisher.Run(ctx); return nil })
	return eg.Wait()
}

// Close освобождает ресурсы сервиса.
func (app *Application) Close() {
	if err := app.storage.Close(); err != nil {
		app.logger.Error().Msg(err.Error())
	}
	if err := app.auditPublisher.Close(); err != nil {
		app.logger.Error().Msg(err.Error())
	}
}

func buildAuditPublisher(cfg config.ServerConfig, logger zerolog.Logger) (*audit.AsyncPublisher, error) {
	publisher := audit.NewAsyncPublisher(logger)
	if cfg.AuditFile != "" {
		o, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			return nil, err
		}
		publisher.Register(o)
	}
	if cfg.AuditURL != "" {
		o := audit.NewHTTPObserver(cfg.AuditURL)
		publisher.Register(o)
	}
	return publisher, nil
}
