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

func NewMemStorage() *MemStorage {
	return &MemStorage{
		metrics: make(map[string]model.Metrics),
	}
}

type FileStorage struct {
	MemStorage
	file *os.File
}

func NewFileStorage(cfg config.ServerConfig) (*FileStorage, error) {
	ms := MemStorage{
		metrics: make(map[string]model.Metrics),
	}
	file, err := os.OpenFile(cfg.FileStoragePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file with metrics: %w", err)
	}
	fs := FileStorage{
		MemStorage: ms,
		file:       file,
	}
	if cfg.Restore {
		if err := restoreMetrics(ms, file); err != nil {
			return nil, err
		}
	}
	return &fs, nil
}

func restoreMetrics(ms MemStorage, file *os.File) error {
	reader := bufio.NewReader(file)
	data, err := io.ReadAll(reader)
	if len(data) == 0 {
		return nil // файл был только что создан пустым, не из чего восстанавливать метрики
	}
	if err != nil {
		return fmt.Errorf("failed to read metrics from the file: %w", err)
	}
	var metricSlice []model.Metrics
	if err := json.Unmarshal(data, &metricSlice); err != nil {
		return fmt.Errorf("failed to unmarshall metrics read from the file: %w", err)
	}
	for _, m := range metricSlice {
		ms.metrics[m.ID] = m
	}
	return nil
}

func (ms *MemStorage) UpdateMetric(m model.Metrics) error {
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

func (ms *MemStorage) GetMetric(mType, mName string) (model.Metrics, error) {
	if v, ok := ms.metrics[mName]; !ok || v.MType != mType {
		return model.Metrics{}, fmt.Errorf("%w: type %s, ID %s", ErrMetricNotFound, mType, mName)
	} else {
		return v, nil
	}
}

func (ms *MemStorage) GetAll() map[string]model.Metrics {
	return ms.metrics
}

func (fs *FileStorage) UpdateMetric(m model.Metrics) error {
	if err := fs.MemStorage.UpdateMetric(m); err != nil {
		return err
	}
	return storeMetrics(fs.MemStorage.GetAll(), fs.file)
}

func storeMetrics(metrics map[string]model.Metrics, file *os.File) error {
	metricSlice := make([]model.Metrics, 0, len(metrics))
	for _, m := range metrics {
		metricSlice = append(metricSlice, m)
	}

	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate the file before storing: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek to the beginning of the file before storing: %w", err)
	}

	writer := bufio.NewWriter(file)
	data, err := json.Marshal(metricSlice)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics before storing: %w", err)
	}
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to write metrics to the file: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush to the file remaining metrics: %w", err)
	}

	return nil
}

func (fs *FileStorage) Close() error {
	return fs.file.Close()
}
