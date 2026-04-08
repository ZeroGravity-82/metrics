package agent

import (
	"testing"

	"zerogravity-82/metrics/internal/model"
)

var (
	sinkBytes     []byte
	sinkErr       error
	sinkMetricMap map[string]model.Metrics
)

func BenchmarkAgent_marshal_updatesBatch(b *testing.B) {
	// Setup
	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 100; i++ {
		v := float64(i)
		metrics = append(metrics, model.Metrics{ID: "m", MType: model.Gauge, Value: &v})
	}
	b.ResetTimer()

	// Measure
	for b.Loop() {
		sinkBytes, sinkErr = marshal(metrics)
	}
}

func BenchmarkAgent_compress_updatesBatch(b *testing.B) {
	// Setup
	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 100; i++ {
		v := float64(i)
		metrics = append(metrics, model.Metrics{ID: "m", MType: model.Gauge, Value: &v})
	}
	jsonBz, err := marshal(metrics)
	if err != nil {
		b.Fatalf("marshal failed: %v", err)
	}
	b.ResetTimer()

	// Measure
	for b.Loop() {
		sinkBytes, sinkErr = compress(jsonBz)
	}
}

func BenchmarkAgent_marshalAndCompress_updatesBatch(b *testing.B) {
	// Setup
	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 100; i++ {
		v := float64(i)
		metrics = append(metrics, model.Metrics{ID: "m", MType: model.Gauge, Value: &v})
	}
	var jsonBz []byte
	b.ResetTimer()

	// Measure
	for b.Loop() {
		jsonBz, sinkErr = marshal(metrics)
		sinkBytes, sinkErr = compress(jsonBz)
	}
}

func BenchmarkAgent_copyMetricsAndResetPollCount(b *testing.B) {
	// Setup
	metrics := newMetrics()
	b.ResetTimer()

	// Measure
	for b.Loop() {
		sinkMetricMap = metrics.copyMetricsAndResetPollCount()
	}
}
