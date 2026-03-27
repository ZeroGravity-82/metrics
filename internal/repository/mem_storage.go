package repository

import (
	"context"
	"fmt"
	"sync"

	"zerogravity-82/metrics/internal/model"
)

// MemStorage - in-memory реализация хранилища метрик.
//
// Реализует интерфейс handler.Storage.
type MemStorage struct {
	mu      sync.Mutex
	metrics map[string]model.Metrics
}

// NewMemStorage создает пустой MemStorage.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		metrics: make(map[string]model.Metrics),
	}
}

// UpdateMetric добавляет или обновляет одну метрику в памяти.
func (ms *MemStorage) UpdateMetric(_ context.Context, m model.Metrics) error {
	if err := validateMetric(m); err != nil {
		return err
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

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

// UpdateMetrics атомарно обновляет несколько метрик в памяти.
//
// Обновление применяется только если все метрики валидны.
func (ms *MemStorage) UpdateMetrics(_ context.Context, metrics []model.Metrics) error {
	for _, m := range metrics {
		if err := validateMetric(m); err != nil {
			return err
		}
	}

	// обновляем только в случае, если все метрики валидные
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, m := range metrics {
		if err := ms.doUpdateMetric(m); err != nil {
			return err
		}
	}

	return nil
}

// GetMetric возвращает метрику по типу и имени.
func (ms *MemStorage) GetMetric(_ context.Context, mType, mName string) (model.Metrics, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	v, ok := ms.metrics[mName]
	if !ok || v.MType != mType {
		return model.Metrics{}, fmt.Errorf("%w: type %s, ID %s", ErrMetricNotFound, mType, mName)
	}
	return v, nil
}

// GetAll возвращает все метрики.
func (ms *MemStorage) GetAll(_ context.Context) (map[string]model.Metrics, error) {
	return ms.metrics, nil
}

// Ping для MemStorage всегда успешный.
func (ms *MemStorage) Ping(_ context.Context) error {
	return nil
}

// Close для MemStorage ничего не делает.
func (ms *MemStorage) Close() error {
	return nil
}
