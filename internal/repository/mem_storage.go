package repository

import (
	"context"
	"fmt"
	"sync"

	"zerogravity-82/metrics/internal/model"
)

type MemStorage struct {
	sync.RWMutex
	metrics map[string]model.Metrics
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		metrics: make(map[string]model.Metrics),
	}
}

func (ms *MemStorage) UpdateMetric(_ context.Context, m model.Metrics) error {
	if err := validateMetric(m); err != nil {
		return err
	}

	ms.Lock()
	defer ms.Unlock()

	return ms.doUpdateMetric(m)
}

func (ms *MemStorage) doUpdateMetric(m model.Metrics) error {
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
	case model.Gauge:
		if existedMetric, ok := ms.metrics[m.ID]; ok {
			if m.MType != existedMetric.MType {
				return fmt.Errorf("%w: %s", ErrInvalidMetricType, m.MType)
			}
		}
		ms.metrics[m.ID] = m
	}
	return nil
}

func (ms *MemStorage) UpdateMetrics(_ context.Context, metrics []model.Metrics) error {
	for _, m := range metrics {
		if err := validateMetric(m); err != nil {
			return err
		}
	}

	// обновляем только в случае, если все метрики валидные
	ms.Lock()
	defer ms.Unlock()

	for _, m := range metrics {
		if err := ms.doUpdateMetric(m); err != nil {
			return err
		}
	}

	return nil
}

func (ms *MemStorage) GetMetric(_ context.Context, mType, mName string) (model.Metrics, error) {
	ms.RLock()
	defer ms.RUnlock()

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
