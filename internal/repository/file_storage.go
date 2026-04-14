package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"zerogravity-82/metrics/internal/model"
)

// FileStorage - персистентное хранилище, которое держит метрики в памяти и при обновлениях сбрасывает их в JSON-файл.
//
// Реализует интерфейс handler.Storage.
type FileStorage struct {
	MemStorage
	file *os.File
	mu   sync.Mutex
}

// NewFileStorage открывает/создает файл по указанному пути и опционально восстанавливает метрики из него.
func NewFileStorage(path string, restore bool) (*FileStorage, error) {
	cleanPath := filepath.Clean(path)
	file, err := os.OpenFile(cleanPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file with metrics: %w", err)
	}
	fs := FileStorage{
		MemStorage: *NewMemStorage(),
		file:       file,
	}
	if restore {
		if err := restoreMetrics(fs.metrics, file); err != nil {
			return nil, err
		}
	}
	return &fs, nil
}

func restoreMetrics(metrics map[string]model.Metrics, file *os.File) error {
	reader := bufio.NewReader(file)
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read metrics from the file: %w", err)
	}
	if len(data) == 0 {
		return nil // файл был только что создан пустым, не из чего восстанавливать метрики
	}
	var metricSlice []model.Metrics
	if err := json.Unmarshal(data, &metricSlice); err != nil {
		return fmt.Errorf("failed to unmarshall metrics read from the file: %w", err)
	}
	for _, m := range metricSlice {
		metrics[m.ID] = m
	}
	return nil
}

// UpdateMetric обновляет одну метрику и сохраняет в файл.
func (fs *FileStorage) UpdateMetric(ctx context.Context, m model.Metrics) error {
	if err := fs.MemStorage.UpdateMetric(ctx, m); err != nil {
		return err
	}
	metrics, err := fs.MemStorage.GetAll(ctx)
	if err != nil {
		return err
	}
	return fs.storeMetrics(metrics)
}

func (fs *FileStorage) storeMetrics(metrics map[string]model.Metrics) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	metricSlice := make([]model.Metrics, 0, len(metrics))
	for _, m := range metrics {
		metricSlice = append(metricSlice, m)
	}

	if err := fs.file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate the file before storing: %w", err)
	}
	if _, err := fs.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek to the beginning of the file before storing: %w", err)
	}

	writer := bufio.NewWriter(fs.file)
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

// UpdateMetrics обновляет несколько метрик и сохраняет в файл.
func (fs *FileStorage) UpdateMetrics(ctx context.Context, metrics []model.Metrics) error {
	if err := fs.MemStorage.UpdateMetrics(ctx, metrics); err != nil {
		return err
	}
	metricsMap, err := fs.MemStorage.GetAll(ctx)
	if err != nil {
		return err
	}
	return fs.storeMetrics(metricsMap)
}

// Ping для FileStorage всегда успешный.
func (fs *FileStorage) Ping(_ context.Context) error {
	return nil
}

// Close закрывает файл хранилища.
func (fs *FileStorage) Close() error {
	return fs.file.Close()
}
