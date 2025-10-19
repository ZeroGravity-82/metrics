package service

import (
	"fmt"

	"zerogravity-82/metrics/internal/model"
)

type Storage interface {
	UpdateMetric(value model.Metric) error
	GetCounter(ID string) (int64, error)
	GetGauge(ID string) (float64, error)
}

type MemStorage struct {
	counters map[string]int64
	gauges   map[string]float64
}

func NewMemStorage() MemStorage {
	return MemStorage{
		counters: make(map[string]int64),
		gauges:   make(map[string]float64),
	}
}

func (ms MemStorage) UpdateMetric(metric model.Metric) error {
	switch metric.MType {
	case model.Counter:
		if metric.Delta == nil {
			return InvalidMetricValueError{
				Message: "counter delta value is invalid",
			}
		}
		if _, ok := ms.counters[metric.ID]; !ok {
			ms.counters[metric.ID] = *metric.Delta
		} else {
			ms.counters[metric.ID] += *metric.Delta
		}
		return nil
	case model.Gauge:
		if metric.Value == nil {
			return InvalidMetricValueError{
				Message: "gauge value is invalid",
			}
		}
		ms.gauges[metric.ID] = *metric.Value
		return nil
	default:
		return UnsupportedMetricTypeError{
			Message: "unsupported metric type",
		}
	}
}

func (ms MemStorage) GetCounter(ID string) (int64, error) {
	if v, ok := ms.counters[ID]; !ok {
		return 0, MetricNotFoundError{
			Message: fmt.Sprintf("counter with ID: %s not found", ID),
		}
	} else {
		return v, nil
	}
}

func (ms MemStorage) GetGauge(ID string) (float64, error) {
	if v, ok := ms.gauges[ID]; !ok {
		return 0.0, MetricNotFoundError{
			Message: fmt.Sprintf("gauge with ID: %s not found", ID),
		}
	} else {
		return v, nil
	}
}
