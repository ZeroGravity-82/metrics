package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"zerogravity-82/metrics/internal/model"
)

// DBStorage - персистентное хранилище метрик на PostgreSQL.
type DBStorage struct {
	db *sqlx.DB
}

// DBMetric - внутреннее представление строки метрики в БД.
//
// ID - наименование (идентификатор) метрики.
//
// MType - тип метрики: счетчик ("counter") или датчик ("gauge").
//
// Delta - значение для счетчика (sql.Null* для датчика).
//
// Value - значение для датчика (sql.Null* для счетчика).
type DBMetric struct {
	ID    string
	MType string `db:"type"`
	Delta sql.NullInt64
	Value sql.NullFloat64
}

// NewDBStorage создает DBStorage поверх существующего sqlx.DB.
func NewDBStorage(db *sqlx.DB) *DBStorage {
	return &DBStorage{db: db}
}

func withRetry(ctx context.Context, fn func() error) func() error {
	const (
		maxRetries        = 3
		firstRetryDelay   = 1 * time.Second
		otherRetriesDelay = 2 * time.Second
	)

	return func() error {
		var err error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			err = fn()
			if err == nil {
				return nil
			}
			if !isRetryableError(err) {
				return err
			}
			if attempt >= maxRetries {
				break
			}
			var timer *time.Timer
			if attempt == 0 {
				timer = time.NewTimer(firstRetryDelay)
			} else {
				timer = time.NewTimer(otherRetriesDelay)
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("operation failed on %d attempt: context timeout: %w", attempt+1, err)
			case <-timer.C:
			}
		}
		return fmt.Errorf("operation failed after %d attempts: %w", maxRetries+1, err)
	}
}

func isRetryableError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.SerializationFailure,
			pgerrcode.DeadlockDetected,
			pgerrcode.TooManyConnections,
			pgerrcode.LockNotAvailable,
			pgerrcode.AdminShutdown,
			pgerrcode.CannotConnectNow,
			pgerrcode.ConnectionException,
			pgerrcode.SQLClientUnableToEstablishSQLConnection,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure:
			return true
		}
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Err.Error() == "connect: connection refused" {
			return true
		}
	}
	return false
}

// UpdateMetric добавляет или обновляет одну метрику.
//
// Операция выполняется с retry при временных ошибках БД/сети.
func (ds *DBStorage) UpdateMetric(ctx context.Context, m model.Metrics) error {
	return withRetry(ctx, func() error {
		return ds.doUpdateMetric(ctx, m)
	})()
}

func (ds *DBStorage) doUpdateMetric(ctx context.Context, m model.Metrics) error {
	if err := validateMetric(m); err != nil {
		return err
	}

	var delta sql.NullInt64
	if m.Delta != nil {
		delta.Int64 = *m.Delta
		delta.Valid = true
	} else {
		delta.Valid = false
	}

	var value sql.NullFloat64
	if m.Value != nil {
		value.Float64 = *m.Value
		value.Valid = true
	} else {
		value.Valid = false
	}

	_, err := ds.db.ExecContext(
		ctx,
		"INSERT INTO metric (id, type, delta, value) VALUES ($1, $2, $3, $4) "+
			"ON CONFLICT (id) DO UPDATE SET delta = metric.delta + EXCLUDED.delta, value = EXCLUDED.value",
		m.ID,
		m.MType,
		delta,
		value,
	)
	if err != nil {
		return fmt.Errorf("failed to update metric: type %s, ID %s: %w", m.MType, m.ID, err)
	}
	return nil
}

// UpdateMetrics добавляет или обновляет набор метрик батчами.
//
// Операция выполняется с retry при временных ошибках БД/сети.
func (ds *DBStorage) UpdateMetrics(ctx context.Context, metrics []model.Metrics) error {
	return withRetry(ctx, func() error {
		return ds.doUpdateMetrics(ctx, metrics)
	})()
}

func (ds *DBStorage) doUpdateMetrics(ctx context.Context, metrics []model.Metrics) error {
	const updateBatchSize = 10

	tx, err := ds.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update metrics: %w", err)
	}
	defer tx.Rollback()

	DBMetricsMap := make(map[string]DBMetric)
	for _, m := range metrics {
		if err = validateMetric(m); err != nil {
			return fmt.Errorf("failed to update metrics: %w", err)
		}

		if _, ok := DBMetricsMap[m.ID]; ok && m.MType == model.Counter {
			*m.Delta += DBMetricsMap[m.ID].Delta.Int64 // Если попался счетчик, надо его инкрементировать
		}

		var delta sql.NullInt64
		if m.Delta != nil {
			delta.Int64 = *m.Delta
			delta.Valid = true
		} else {
			delta.Valid = false
		}

		var value sql.NullFloat64
		if m.Value != nil {
			value.Float64 = *m.Value
			value.Valid = true
		} else {
			value.Valid = false
		}

		DBMetricsMap[m.ID] = DBMetric{ID: m.ID, MType: m.MType, Delta: delta, Value: value}
		if len(DBMetricsMap) >= updateBatchSize {
			DBMetricsSlice := buildSortedDBMetricsSlice(DBMetricsMap)
			_, err = ds.db.NamedExec("INSERT INTO metric (id, type, delta, value) VALUES (:id, :type, :delta, :value) "+
				"ON CONFLICT (id) DO UPDATE SET delta = metric.delta + EXCLUDED.delta, value = EXCLUDED.value", DBMetricsSlice)
			if err != nil {
				return fmt.Errorf("failed to update metrics: %w", err)
			}
			DBMetricsMap = make(map[string]DBMetric)
		}
	}
	if len(DBMetricsMap) > 0 {
		DBMetricsSlice := buildSortedDBMetricsSlice(DBMetricsMap)
		_, err = ds.db.NamedExec("INSERT INTO metric (id, type, delta, value) VALUES (:id, :type, :delta, :value) "+
			"ON CONFLICT (id) DO UPDATE SET delta = metric.delta + EXCLUDED.delta, value = EXCLUDED.value", DBMetricsSlice)
		if err != nil {
			return fmt.Errorf("failed to update metrics: %w", err)
		}
	}
	return tx.Commit()
}

func buildSortedDBMetricsSlice(metricsMap map[string]DBMetric) []DBMetric {
	metricsIds := make([]string, 0, len(metricsMap))
	for id := range metricsMap {
		metricsIds = append(metricsIds, id)
	}
	slices.Sort(metricsIds)

	metricsSlice := make([]DBMetric, 0, len(metricsMap))
	for _, id := range metricsIds {
		metricsSlice = append(metricsSlice, metricsMap[id])
	}
	return metricsSlice
}

// GetMetric возвращает метрику по типу и имени (идентификатору) из БД.
//
// Операция выполняется с retry при временных ошибках БД/сети.
func (ds *DBStorage) GetMetric(ctx context.Context, mType, mName string) (model.Metrics, error) {
	var m model.Metrics
	err := withRetry(ctx, func() error {
		var err error
		m, err = ds.doGetMetric(ctx, mType, mName)
		return err
	})()
	return m, err
}

func (ds *DBStorage) doGetMetric(ctx context.Context, mType, mName string) (model.Metrics, error) {
	if mType != model.Counter && mType != model.Gauge {
		return model.Metrics{}, fmt.Errorf("%w: %s", ErrUnsupportedMetricType, mType)
	}
	var m DBMetric
	err := ds.db.GetContext(
		ctx,
		&m,
		"SELECT id, type, delta, value FROM metric WHERE id = $1 AND type = $2",
		mName,
		mType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Metrics{}, fmt.Errorf("%w: type %s, ID %s", ErrMetricNotFound, mType, mName)
	}
	if err != nil {
		return model.Metrics{}, err
	}

	if m.MType == model.Counter {
		return model.Metrics{ID: m.ID, MType: m.MType, Delta: &m.Delta.Int64, Value: nil}, nil
	}
	return model.Metrics{ID: m.ID, MType: m.MType, Delta: nil, Value: &m.Value.Float64}, nil
}

// GetAll возвращает все метрики из БД.
//
// Операция выполняется с retry при временных ошибках БД/сети.
func (ds *DBStorage) GetAll(ctx context.Context) (map[string]model.Metrics, error) {
	var metrics map[string]model.Metrics
	err := withRetry(ctx, func() error {
		var err error
		metrics, err = ds.doGetAll(ctx)
		return err
	})()
	return metrics, err
}

func (ds *DBStorage) doGetAll(ctx context.Context) (map[string]model.Metrics, error) {
	var metrics = make([]DBMetric, 0)

	if err := ds.db.SelectContext(ctx, &metrics, "SELECT id, type, delta, value FROM metric"); err != nil {
		return nil, fmt.Errorf("failed query all metrics from DB: %w", err)
	}
	metricsMap := make(map[string]model.Metrics, len(metrics))
	for _, m := range metrics {
		if m.MType == model.Counter {
			metricsMap[m.ID] = model.Metrics{
				ID:    m.ID,
				MType: m.MType,
				Delta: &m.Delta.Int64,
				Value: nil,
			}
		}
		if m.MType == model.Gauge {
			metricsMap[m.ID] = model.Metrics{
				ID:    m.ID,
				MType: m.MType,
				Delta: nil,
				Value: &m.Value.Float64,
			}
		}
	}
	return metricsMap, nil
}

// Ping проверяет доступность подключения к БД.
func (ds *DBStorage) Ping(ctx context.Context) error {
	return ds.db.PingContext(ctx)
}

// Close закрывает соединение с БД.
func (ds *DBStorage) Close() error {
	return ds.db.Close()
}
