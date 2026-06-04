package repository

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"zerogravity-82/metrics/internal/model"
)

var (
	sinkErr    error
	sinkMetric model.Metrics
)

func BenchmarkMemStorage_UpdateMetric(b *testing.B) {
	// Setup
	ms := NewMemStorage()
	ctx := context.Background()

	d := int64(1)
	m := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &d}

	// Measure
	for b.Loop() {
		sinkErr = ms.UpdateMetric(ctx, m)
	}
}

func BenchmarkMemStorage_UpdateMetrics(b *testing.B) {
	// Setup
	ms := NewMemStorage()
	ctx := context.Background()

	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 50; i++ {
		v := float64(i)
		d := int64(i)
		metrics = append(metrics, model.Metrics{ID: "m" + strconv.Itoa(i), MType: model.Gauge, Value: &v})
		metrics = append(metrics, model.Metrics{ID: "c" + strconv.Itoa(i), MType: model.Counter, Delta: &d})
	}

	// Measure
	for b.Loop() {
		sinkErr = ms.UpdateMetrics(ctx, metrics)
	}
}

func BenchmarkMemStorage_GetMetric(b *testing.B) {
	// Setup
	ms := NewMemStorage()
	ctx := context.Background()

	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 50; i++ {
		v := float64(i)
		d := int64(i)
		metrics = append(metrics, model.Metrics{ID: "m" + strconv.Itoa(i), MType: model.Gauge, Value: &v})
		metrics = append(metrics, model.Metrics{ID: "c" + strconv.Itoa(i), MType: model.Counter, Delta: &d})
	}
	sinkErr = ms.UpdateMetrics(ctx, metrics)
	v := float64(123.45)
	sinkErr = ms.UpdateMetric(ctx, model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &v})

	// Measure
	for b.Loop() {
		sinkMetric, sinkErr = ms.GetMetric(ctx, model.Gauge, "RandomValue")
	}
}

func BenchmarkFileStorage_UpdateMetric(b *testing.B) {
	// Setup
	ctx := context.Background()

	dir := b.TempDir()
	path := filepath.Join(dir, "metrics.json")
	fs, err := NewFileStorage(path, false)
	if err != nil {
		b.Fatalf("NewFileStorage failed: %v", err)
	}
	b.Cleanup(func() {
		_ = fs.Close()
	})

	v := float64(123.45)
	m := model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &v}

	// Measure
	for b.Loop() {
		sinkErr = fs.UpdateMetric(ctx, m)
	}
}

func BenchmarkFileStorage_UpdateMetrics(b *testing.B) {
	// Setup
	ctx := context.Background()

	dir := b.TempDir()
	path := filepath.Join(dir, "metrics.json")
	fs, err := NewFileStorage(path, false)
	if err != nil {
		b.Fatalf("NewFileStorage failed: %v", err)
	}
	b.Cleanup(func() {
		_ = fs.Close()
	})

	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 50; i++ {
		v := float64(i)
		d := int64(i)
		metrics = append(metrics, model.Metrics{ID: "m" + strconv.Itoa(i), MType: model.Gauge, Value: &v})
		metrics = append(metrics, model.Metrics{ID: "c" + strconv.Itoa(i), MType: model.Counter, Delta: &d})
	}

	// Measure
	for b.Loop() {
		sinkErr = fs.UpdateMetrics(ctx, metrics)
	}
}

func BenchmarkFileStorage_GetMetric(b *testing.B) {
	// Setup
	ctx := context.Background()

	dir := b.TempDir()
	path := filepath.Join(dir, "metrics.json")
	fs, err := NewFileStorage(path, false)
	if err != nil {
		b.Fatalf("NewFileStorage failed: %v", err)
	}
	b.Cleanup(func() {
		_ = fs.Close()
	})

	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 50; i++ {
		v := float64(i)
		d := int64(i)
		metrics = append(metrics, model.Metrics{ID: "m" + strconv.Itoa(i), MType: model.Gauge, Value: &v})
		metrics = append(metrics, model.Metrics{ID: "c" + strconv.Itoa(i), MType: model.Counter, Delta: &d})
	}
	sinkErr = fs.UpdateMetrics(ctx, metrics)
	v := float64(123.45)
	sinkErr = fs.UpdateMetric(ctx, model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &v})

	// Measure
	for b.Loop() {
		sinkMetric, sinkErr = fs.GetMetric(ctx, model.Gauge, "RandomValue")
	}
}

func BenchmarkFileStorage_RestoreMetrics(b *testing.B) {
	// Setup
	ctx := context.Background()

	dir := b.TempDir()
	path := filepath.Join(dir, "metrics.json")
	fs, err := NewFileStorage(path, false)
	if err != nil {
		b.Fatalf("NewFileStorage failed: %v", err)
	}

	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 100; i++ {
		v := float64(i)
		metrics = append(metrics, model.Metrics{ID: "m", MType: model.Gauge, Value: &v})
	}
	if err := fs.UpdateMetrics(ctx, metrics); err != nil {
		b.Fatalf("UpdateMetrics failed: %v", err)
	}
	_ = fs.Close()

	// Measure
	for b.Loop() {
		b.StopTimer()
		// #nosec G304 -- path is from b.TempDir(), not user input
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			b.Fatalf("open failed: %v", err)
		}
		metricsMap := make(map[string]model.Metrics, 100)
		b.StartTimer()

		sinkErr = restoreMetrics(metricsMap, f)
		_ = f.Close()
	}
}
