// Пакет application собирает и запускает основные компоненты сервиса.
package application

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/grpcserver"
	"zerogravity-82/metrics/internal/httpserver"
	"zerogravity-82/metrics/internal/repository"
	"zerogravity-82/metrics/internal/service/audit"
)

// Storage абстрагирует хранилище метрик.
type Storage interface {
	Close() error
}

type metricStorage interface {
	Storage
	httpserver.Storage
	grpcserver.Storage
}

// Application связывает конфигурацию, хранилище, аудитора запросов и HTTP/gRPC-сервера в единый сервис.
//
// Используется в cmd/server для сборки и запуска сервиса.
type Application struct {
	logger         zerolog.Logger
	cfg            config.ServerConfig
	storage        Storage
	httpSrv        *httpserver.HTTPServer
	grpcSrv        *grpcserver.GRPCServer
	pprofSrv       *httpserver.PprofServer
	auditPublisher *audit.AsyncPublisher
}

// NewApplication собирает Application с учетом настроек из переменных окружения и флагов.
func NewApplication(logger zerolog.Logger) (*Application, error) {
	cfg, err := config.GetServerConfig()
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	storage, err := buildStorage(cfg)
	if err != nil {
		return nil, fmt.Errorf("storage error: %w", err)
	}

	publisher, err := buildAuditPublisher(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("audit publisher error: %w", err)
	}

	tlsCert, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tls certificate: %w", err)
	}
	httpSrv := buildHTTPServer(
		cfg.HTTPServerAddr,
		tlsCert,
		cfg.SignatureKey,
		cfg.CryptoKeyPath,
		cfg.TrustedSubnet,
		storage,
		publisher,
		logger,
	)
	grpcSrv := buildGRPCServer(cfg.GRPCServerAddr, tlsCert, cfg.TrustedSubnet, storage, publisher, logger)
	pprofSrv := httpserver.NewPprofServer(cfg.PprofAddr, logger)

	return &Application{
		logger:         logger,
		cfg:            cfg,
		storage:        storage,
		httpSrv:        httpSrv,
		grpcSrv:        grpcSrv,
		pprofSrv:       pprofSrv,
		auditPublisher: publisher,
	}, nil
}

func buildStorage(cfg config.ServerConfig) (metricStorage, error) {
	var (
		storage metricStorage
		err     error
	)
	switch {
	case cfg.DatabaseDSN != "":
		var db *sqlx.DB
		db, err = sqlx.Connect("pgx", cfg.DatabaseDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to the database: %w", err)
		}
		if err = applyMigrations(db); err != nil {
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
	return storage, nil
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

func buildHTTPServer(
	httpServerAddr string,
	tlsCert tls.Certificate,
	signatureKey string,
	cryptoKeyPath string,
	trustedSubnet string,
	storage metricStorage,
	publisher *audit.AsyncPublisher,
	logger zerolog.Logger,
) *httpserver.HTTPServer {
	httpTlsConfig := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	httpSrv := httpserver.NewHTTPServer(
		httpServerAddr,
		httpTlsConfig,
		storage,
		publisher,
		signatureKey,
		cryptoKeyPath,
		trustedSubnet,
		logger,
	)
	return httpSrv
}

func buildGRPCServer(
	grpcServerAddr string,
	tlsCert tls.Certificate,
	trustedSubnet string,
	storage metricStorage,
	publisher *audit.AsyncPublisher,
	logger zerolog.Logger,
) *grpcserver.GRPCServer {
	grpcTlsConfig := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	grpcSrvCredentials := credentials.NewTLS(grpcTlsConfig)
	grpcSrv := grpcserver.NewGRPCServer(
		grpcServerAddr,
		grpcSrvCredentials,
		storage,
		publisher,
		trustedSubnet,
		logger,
	)
	return grpcSrv
}

// Run запускает основные подсистемы сервиса и блокируется, пока не отменен контекст или один из серверов не
// остановится с ошибкой.
func (app *Application) Run(ctx context.Context) error {
	serverErrCh := app.runServers(ctx)
	auditErrCh := app.runAuditPublisher()

	select {
	case <-ctx.Done():
		serverErr := <-serverErrCh // блокируемся до завершения работы группы серверов
		auditErr := app.shutdownAuditPublisher(auditErrCh)
		return errors.Join(serverErr, auditErr)
	case serverErr := <-serverErrCh:
		auditErr := app.shutdownAuditPublisher(auditErrCh)
		return errors.Join(serverErr, auditErr)
	}
}

func (app *Application) runServers(ctx context.Context) <-chan error {
	serverErrCh := make(chan error, 1)
	eg, groupCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return app.httpSrv.Run(groupCtx)
	})
	eg.Go(func() error {
		return app.grpcSrv.Run(groupCtx)
	})
	eg.Go(func() error {
		return app.pprofSrv.Run(groupCtx)
	})
	go func() {
		serverErrCh <- eg.Wait()
	}()
	return serverErrCh
}

func (app *Application) runAuditPublisher() <-chan error {
	auditErrCh := make(chan error, 1)
	go func() {
		auditErrCh <- app.auditPublisher.Run()
	}()
	return auditErrCh
}

func (app *Application) shutdownAuditPublisher(auditErrCh <-chan error) error {
	const shutdownTimeout = 10 * time.Second

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	app.auditPublisher.Shutdown(shutdownCtx)
	return <-auditErrCh
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
