package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"zerogravity-82/metrics/internal/config"

	"zerogravity-82/metrics/internal/model"
)

var ErrMetricNotFound = errors.New("metric not found")
var ErrInvalidMetricValue = errors.New("invalid metric value")
var ErrInvalidMetricType = errors.New("invalid metric type")
var ErrUnsupportedMetricType = errors.New("unsupported metric type")

type MemStorage struct {
	metrics map[string]model.Metrics
}

func NewMemStorage(cfg config.ServerConfig) (*MemStorage, error) {
	ms := MemStorage{
		metrics: make(map[string]model.Metrics),
	}
	if cfg.Restore {
		if err := restoreMetrics(ms, cfg.FileStoragePath); err != nil {
			return nil, err
		}
	}
	return &ms, nil
}

func restoreMetrics(ms MemStorage, filename string) error {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0666)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	reader := bufio.NewReader(file)
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	var metricSlice []model.Metrics
	if err := json.Unmarshal(data, &metricSlice); err != nil {
		return err
	}
	for _, m := range metricSlice {
		ms.metrics[m.ID] = m
	}
	return nil
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
