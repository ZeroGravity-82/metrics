package service

import (
	"fmt"
	"strconv"

	"zerogravity-82/metrics/internal/model"
)

type Storage interface {
	UpdateMetric(mType, mName, mValue string) error
	GetMetric(ID string) (model.Metric, error)
}

type MemStorage struct {
	metrics map[string]model.Metric
}

func NewMemStorage() MemStorage {
	return MemStorage{
		metrics: make(map[string]model.Metric),
	}
}

func (ms MemStorage) UpdateMetric(mType, mName, mValue string) error {
	switch mType {
	case model.Counter:
		v, err := strconv.ParseInt(mValue, 10, 64)
		if err != nil {
			return InvalidMetricValueError{
				Message: "counter delta value is invalid",
			}
		}
		metric := model.Metric{
			ID:    mName,
			MType: mType,
			Delta: &v,
			Value: nil,
			Hash:  "",
		}
		if _, ok := ms.metrics[metric.ID]; !ok {
			ms.metrics[metric.ID] = metric
		} else {
			*ms.metrics[metric.ID].Delta += *metric.Delta
		}
		return nil
	case model.Gauge:
		v, err := strconv.ParseFloat(mValue, 64)
		if err != nil {
			return InvalidMetricValueError{
				Message: "gauge value is invalid",
			}
		}
		metric := model.Metric{
			ID:    mName,
			MType: mType,
			Delta: nil,
			Value: &v,
			Hash:  "",
		}
		ms.metrics[metric.ID] = metric
		return nil
	default:
		return UnsupportedMetricTypeError{
			Message: "unsupported metric type",
		}
	}

}

func (ms MemStorage) GetMetric(ID string) (model.Metric, error) {
	if v, ok := ms.metrics[ID]; !ok {
		return model.Metric{}, MetricNotFoundError{
			Message: fmt.Sprintf("metric with ID: %s not found", ID),
		}
	} else {
		return v, nil
	}
}
