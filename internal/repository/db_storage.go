package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"zerogravity-82/metrics/internal/model"
)

type DBStorage struct {
	db *sql.DB
}

func NewDbStorage(db *sql.DB) *DBStorage {
	return &DBStorage{db: db}
}

func (ds *DBStorage) UpdateMetric(ctx context.Context, m model.Metrics) error {
	_, err := ds.db.ExecContext(
		ctx,
		"INSERT INTO counter (id, delta) VALUES ($1, $2) ON CONFLICT DO UPDATE SET delta = $2",
		m.ID,
		*m.Delta,
	)
	if err != nil {
		return fmt.Errorf("failed to update metric with type counter, ID %s, value %d in DB: %w", m.ID, *m.Delta, err)
	}
	return nil
}

func (ds *DBStorage) GetMetric(ctx context.Context, mType, mName string) (model.Metrics, error) {
	row := ds.db.QueryRowContext(ctx, "SELECT id, delta FROM counter WHERE id = $1", mName)
	var id string
	var delta int64
	if err := row.Scan(&id, &delta); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Metrics{}, fmt.Errorf("%w: type %s, ID %s", ErrMetricNotFound, mType, mName)
		}
	}
	return model.Metrics{
		ID:    id,
		MType: model.Counter,
		Delta: &delta,
		Value: nil,
		Hash:  "",
	}, nil
}

func (ds *DBStorage) GetAll(ctx context.Context) (map[string]model.Metrics, error) {
	rows, err := ds.db.QueryContext(ctx, "SELECT id, delta FROM counter")
	if err != nil {
		return nil, fmt.Errorf("failed to query all metrics from DB: %w", err)
	}
	defer rows.Close()

	var id string
	var delta int64
	metrics := make(map[string]model.Metrics)
	for rows.Next() {
		if err = rows.Scan(&id, &delta); err != nil {
			return nil, fmt.Errorf("failed to scan metric from DB: %w", err)
		}
		metrics[id] = model.Metrics{
			ID:    id,
			MType: model.Counter,
			Delta: &delta,
			Value: nil,
			Hash:  "",
		}
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("failed to iterate over metrics queried from DB: %w", err)
	}
	return metrics, nil
}
