package service

import (
	"fmt"
	"strconv"

	"zerogravity-82/metrics/internal/model"
)

type Storage interface {
	UpdateMetric(mType, mName, mValue string) error
	GetCounterMetric(ID string) (model.CounterMetric, error)
	GetGaugeMetric(ID string) (model.GaugeMetric, error)
	GetAll() ([]model.CounterMetric, []model.GaugeMetric)
}

type MemStorage struct {
	counters map[string]model.CounterMetric
	gauges   map[string]model.GaugeMetric
}

func NewMemStorage() MemStorage {
	return MemStorage{
		counters: make(map[string]model.CounterMetric),
		gauges:   make(map[string]model.GaugeMetric),
	}
}

func (ms MemStorage) UpdateMetric(mType, mName, mValue string) error {
	if len(mName) == 0 {
		return MetricNotFoundError{
			Message: "metric with empty name",
		}
	}

	switch mType {
	case model.Counter:
		v, err := strconv.ParseInt(mValue, 10, 64)
		if err != nil {
			return InvalidMetricValueError{
				Message: "counter delta value is invalid",
			}
		}
		m := model.CounterMetric{
			Metric: model.Metric{
				ID:    mName,
				MType: mType,
				Hash:  "",
			},
			Delta: v,
		}
		if _, ok := ms.counters[m.ID]; !ok {
			ms.counters[m.ID] = m
		} else {
			existedMetric := ms.counters[m.ID]
			existedMetric.Delta += m.Delta
			ms.counters[m.ID] = existedMetric
		}
		return nil
	case model.Gauge:
		v, err := strconv.ParseFloat(mValue, 64)
		if err != nil {
			return InvalidMetricValueError{
				Message: "gauge value is invalid",
			}
		}
		metric := model.GaugeMetric{
			Metric: model.Metric{
				ID:    mName,
				MType: mType,
				Hash:  "",
			},
			Value: v,
		}
		ms.gauges[metric.ID] = metric
		return nil
	default:
		return UnsupportedMetricTypeError{
			Message: "unsupported metric type",
		}
	}

}

func (ms MemStorage) GetCounterMetric(ID string) (model.CounterMetric, error) {
	if v, ok := ms.counters[ID]; !ok {
		return model.CounterMetric{}, MetricNotFoundError{
			Message: fmt.Sprintf("counter metric with ID: %s not found", ID),
		}
	} else {
		return v, nil
	}
}

func (ms MemStorage) GetGaugeMetric(ID string) (model.GaugeMetric, error) {
	if v, ok := ms.gauges[ID]; !ok {
		return model.GaugeMetric{}, MetricNotFoundError{
			Message: fmt.Sprintf("gauge metric with ID: %s not found", ID),
		}
	} else {
		return v, nil
	}
}

func (ms MemStorage) GetAll() ([]model.CounterMetric, []model.GaugeMetric) {
	counters := make([]model.CounterMetric, 0, len(ms.counters))
	gauges := make([]model.GaugeMetric, 0, len(ms.gauges))
	for _, v := range ms.counters {
		counters = append(counters, v)
	}
	for _, v := range ms.gauges {
		gauges = append(gauges, v)
	}
	return counters, gauges
}
