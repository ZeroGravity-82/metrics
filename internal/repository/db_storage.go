package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"zerogravity-82/metrics/internal/model"
)

type DBStorage struct {
	db *sql.DB
}

func NewDbStorage(db *sql.DB) *DBStorage {
	return &DBStorage{db: db}
}

func (ds *DBStorage) UpdateMetric(ctx context.Context, m model.Metrics) error {
	if len(m.ID) == 0 {
		return fmt.Errorf("%w: empty name", ErrMetricNotFound)
	}
	if m.MType == model.Counter && (m.Delta == nil || m.Value != nil) ||
		m.MType == model.Gauge && (m.Value == nil || m.Delta != nil) {
		return fmt.Errorf("%w", ErrInvalidMetricValue)
	}
	if m.MType != model.Counter && m.MType != model.Gauge {
		return fmt.Errorf("%w: %s", ErrUnsupportedMetricType, m.MType)
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

func (ds *DBStorage) GetMetric(ctx context.Context, mType, mName string) (model.Metrics, error) {
	if mType != model.Counter && mType != model.Gauge {
		return model.Metrics{}, fmt.Errorf("%w: %s", ErrUnsupportedMetricType, mType)
	}
	row := ds.db.QueryRowContext(
		ctx,
		"SELECT id, type, delta, value FROM metric WHERE id = $1 AND type = $2", mName, mType,
	)
	var (
		m     model.Metrics
		delta sql.NullInt64
		value sql.NullFloat64
	)
	if err := row.Scan(&m.ID, &m.MType, &delta, &value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Metrics{}, fmt.Errorf("%w: type %s, ID %s", ErrMetricNotFound, mType, mName)
		}
	}
	if delta.Valid == true {
		m.Delta = &delta.Int64
	} else {
		m.Delta = nil
	}
	if value.Valid == true {
		m.Value = &value.Float64
	} else {
		m.Value = nil
	}
	return m, nil
}

func (ds *DBStorage) GetAll(ctx context.Context) (map[string]model.Metrics, error) {
	rows, err := ds.db.QueryContext(ctx, "SELECT id, type, delta, value FROM metric")
	if err != nil {
		return nil, fmt.Errorf("failed to query all metrics from DB: %w", err)
	}
	defer rows.Close()

	var (
		m     model.Metrics
		delta sql.NullInt64
		value sql.NullFloat64
	)
	metrics := make(map[string]model.Metrics)
	for rows.Next() {
		if err = rows.Scan(&m.ID, &m.MType, &delta, &value); err != nil {
			return nil, fmt.Errorf("failed to scan the metric from DB: %w", err)
		}
		if delta.Valid == true {
			m.Delta = &delta.Int64
		} else {
			m.Delta = nil
		}
		if value.Valid == true {
			m.Value = &value.Float64
		} else {
			m.Value = nil
		}
		metrics[m.ID] = m
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("failed to iterate over metrics queried from DB: %w", err)
	}
	return metrics, nil
}

func (ds *DBStorage) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := ds.db.PingContext(ctx); err != nil {
		return err
	}
	return nil
}

func (ds *DBStorage) Close() error {
	return ds.db.Close()
}
