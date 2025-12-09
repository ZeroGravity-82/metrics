package repository

import (
	"context"
	"fmt"

	"zerogravity-82/metrics/internal/model"
)

type MemStorage struct {
	metrics map[string]model.Metrics
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		metrics: make(map[string]model.Metrics),
	}
}

func (ms *MemStorage) UpdateMetric(_ context.Context, m model.Metrics) error {
	if len(m.ID) == 0 {
		return fmt.Errorf("%w: empty name", ErrMetricNotFound)
	}
	if m.MType == model.Counter && (m.Delta == nil || m.Value != nil) ||
		m.MType == model.Gauge && (m.Value == nil || m.Delta != nil) {
		return fmt.Errorf("%w", ErrInvalidMetricValue)
	}

	switch m.MType {
	case model.Counter:
		if _, ok := ms.metrics[m.ID]; !ok {
			ms.metrics[m.ID] = m
		} else {
			existedMetric := ms.metrics[m.ID]
			if m.MType != existedMetric.MType {
				return fmt.Errorf("%w: %s", ErrInvalidMetricType, m.MType)
			}
			*existedMetric.Delta += *m.Delta
			ms.metrics[m.ID] = existedMetric
		}
		return nil
	case model.Gauge:
		if existedMetric, ok := ms.metrics[m.ID]; ok {
			if m.MType != existedMetric.MType {
				return fmt.Errorf("%w: %s", ErrInvalidMetricType, m.MType)
			}
		}
		ms.metrics[m.ID] = m
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedMetricType, m.MType)
	}
}

func (ms *MemStorage) GetMetric(_ context.Context, mType, mName string) (model.Metrics, error) {
	if v, ok := ms.metrics[mName]; !ok || v.MType != mType {
		return model.Metrics{}, fmt.Errorf("%w: type %s, ID %s", ErrMetricNotFound, mType, mName)
	} else {
		return v, nil
	}
}

func (ms *MemStorage) GetAll(_ context.Context) (map[string]model.Metrics, error) {
	return ms.metrics, nil
}

func (ms *MemStorage) Ping(_ context.Context) error {
	return nil
}

func (ms *MemStorage) Close() error {
	return nil
}
