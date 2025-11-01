package service

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"zerogravity-82/metrics/internal/model"
)

var ErrMetricNotFound = errors.New("metric not found")
var ErrInvalidMetricValue = errors.New("invalid metric value")
var ErrUnsupportedMetricType = errors.New("unsupported metric type")

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
		return fmt.Errorf("%w: empty name", ErrMetricNotFound)
	}

	switch mType {
	case model.Counter:
		v, err := strconv.ParseInt(mValue, 10, 64)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidMetricValue, mValue)
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
			return fmt.Errorf("%w: %s", ErrInvalidMetricValue, mValue)
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
		return fmt.Errorf("%w: %s", ErrUnsupportedMetricType, mType)
	}
}

func (ms MemStorage) GetCounterMetric(ID string) (model.CounterMetric, error) {
	if v, ok := ms.counters[ID]; !ok {
		return model.CounterMetric{}, fmt.Errorf("%w: counter with ID %s", ErrMetricNotFound, ID)
	} else {
		return v, nil
	}
}

func (ms MemStorage) GetGaugeMetric(ID string) (model.GaugeMetric, error) {
	if v, ok := ms.gauges[ID]; !ok {
		return model.GaugeMetric{}, fmt.Errorf("%w: gauge with ID %s", ErrMetricNotFound, ID)
	} else {
		return v, nil
	}
}

func (ms MemStorage) GetAll() ([]model.CounterMetric, []model.GaugeMetric) {
	return getCounters(ms), getGauges(ms)
}

func getCounters(ms MemStorage) []model.CounterMetric {
	counterNames := make([]string, 0, len(ms.counters))
	for n := range ms.counters {
		counterNames = append(counterNames, n)
	}
	slices.Sort(counterNames)
	counters := make([]model.CounterMetric, 0, len(ms.counters))
	for _, n := range counterNames {
		counters = append(counters, ms.counters[n])
	}
	return counters
}

func getGauges(ms MemStorage) []model.GaugeMetric {
	gaugeNames := make([]string, 0, len(ms.gauges))
	for n := range ms.gauges {
		gaugeNames = append(gaugeNames, n)
	}
	slices.Sort(gaugeNames)
	gauges := make([]model.GaugeMetric, 0, len(ms.gauges))
	for _, n := range gaugeNames {
		gauges = append(gauges, ms.gauges[n])
	}
	return gauges
}
