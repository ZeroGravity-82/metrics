package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/jmoiron/sqlx"

	"zerogravity-82/metrics/internal/model"
)

const (
	updateBatchSize = 10
)

type DBStorage struct {
	db *sqlx.DB
}

type DBMetric struct {
	ID    string
	MType string `db:"type"`
	Delta sql.NullInt64
	Value sql.NullFloat64
}

func NewDbStorage(db *sqlx.DB) *DBStorage {
	return &DBStorage{db: db}
}

func (ds *DBStorage) UpdateMetric(ctx context.Context, m model.Metrics) error {
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

func (ds *DBStorage) UpdateMetrics(ctx context.Context, metrics []model.Metrics) error {
	tx, err := ds.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update metrics: %w", err)
	}
	defer tx.Rollback()

	DBMetricsMap := make(map[string]DBMetric)
	for _, m := range metrics {
		if err := validateMetric(m); err != nil {
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
			DBMetricsSlice := buildSortedDbMetricsSlice(DBMetricsMap)
			_, err = ds.db.NamedExec("INSERT INTO metric (id, type, delta, value) VALUES (:id, :type, :delta, :value) "+
				"ON CONFLICT (id) DO UPDATE SET delta = metric.delta + EXCLUDED.delta, value = EXCLUDED.value", DBMetricsSlice)
			if err != nil {
				return fmt.Errorf("failed to update metrics: %w", err)
			}
			DBMetricsMap = make(map[string]DBMetric)
		}
	}
	if len(DBMetricsMap) > 0 {
		DBMetricsSlice := buildSortedDbMetricsSlice(DBMetricsMap)
		_, err = ds.db.NamedExec("INSERT INTO metric (id, type, delta, value) VALUES (:id, :type, :delta, :value) "+
			"ON CONFLICT (id) DO UPDATE SET delta = metric.delta + EXCLUDED.delta, value = EXCLUDED.value", DBMetricsSlice)
		if err != nil {
			return fmt.Errorf("failed to update metrics: %w", err)
		}
	}
	return tx.Commit()
}

func buildSortedDbMetricsSlice(metricsMap map[string]DBMetric) []DBMetric {
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

func (ds *DBStorage) GetMetric(ctx context.Context, mType, mName string) (model.Metrics, error) {
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

func (ds *DBStorage) GetAll(ctx context.Context) (map[string]model.Metrics, error) {
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

func (ds *DBStorage) Ping(ctx context.Context) error {
	return ds.db.PingContext(ctx)
}

func (ds *DBStorage) Close() error {
	return ds.db.Close()
}
