package service

import (
	"errors"
	"fmt"
	"strconv"

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
		m := model.Metrics{
			ID:    mName,
			MType: mType,
			Delta: &v,
			Value: nil,
			Hash:  "",
		}
		if _, ok := ms.metrics[m.ID]; !ok {
			ms.metrics[m.ID] = m
		} else {
			existedMetric := ms.metrics[m.ID]
			if mType != existedMetric.MType {
				return fmt.Errorf("%w: %s", ErrInvalidMetricType, mType)
			}
			*existedMetric.Delta += *m.Delta
			ms.metrics[m.ID] = existedMetric
		}
		return nil
	case model.Gauge:
		v, err := strconv.ParseFloat(mValue, 64)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidMetricValue, mValue)
		}
		m := model.Metrics{
			ID:    mName,
			MType: mType,
			Delta: nil,
			Value: &v,
			Hash:  "",
		}
		if existedMetric, ok := ms.metrics[m.ID]; ok {
			if mType != existedMetric.MType {
				return fmt.Errorf("%w: %s", ErrInvalidMetricType, mType)
			}
		}
		ms.metrics[m.ID] = m
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedMetricType, mType)
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
