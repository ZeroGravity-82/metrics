package service

import (
	"errors"
	"fmt"

	"zerogravity-82/metrics/internal/model"
)

var ErrMetricNotFound = errors.New("metric not found")
var ErrInvalidMetricValue = errors.New("invalid metric value")
var ErrInvalidMetricType = errors.New("invalid metric type")
var ErrUnsupportedMetricType = errors.New("unsupported metric type")

type MemStorage struct {
	metrics map[string]model.Metrics
}

func NewMemStorage() MemStorage {
	return MemStorage{
		metrics: make(map[string]model.Metrics),
	}
}

func (ms MemStorage) UpdateMetric(m model.Metrics) error {
	if len(m.ID) == 0 {
		return fmt.Errorf("%w: empty name", ErrMetricNotFound)
	}
	if m.MType == model.Counter && m.Delta == nil ||
		m.MType == model.Gauge && m.Value == nil {
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

func (ms MemStorage) GetMetric(mType, mName string) (model.Metrics, error) {
	if v, ok := ms.metrics[mName]; !ok || v.MType != mType {
		return model.Metrics{}, fmt.Errorf("%w: type %s, ID %s", ErrMetricNotFound, mType, mName)
	} else {
		return v, nil
	}
}

func (ms MemStorage) GetAll() map[string]model.Metrics {
	return ms.metrics
}
