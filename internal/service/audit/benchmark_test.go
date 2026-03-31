package audit

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/model"
)

type nopObserver struct{}

func (nopObserver) update(context.Context, model.AuditLog) error { return nil }

var sinkAuditLog model.AuditLog

func BenchmarkAsyncPublisher_convertMetricsToAuditLog(b *testing.B) {
	// Setup
	now := time.Unix(1774200253, 0)
	ip := "127.0.0.1"

	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 50; i++ {
		v := float64(i)
		d := int64(i)
		metrics = append(metrics, model.Metrics{ID: "m" + strconv.Itoa(i), MType: model.Gauge, Value: &v})
		metrics = append(metrics, model.Metrics{ID: "c" + strconv.Itoa(i), MType: model.Counter, Delta: &d})
	}
	b.ResetTimer()

	// Measure
	for b.Loop() {
		sinkAuditLog = convertMetricsToAuditLog(now, ip, metrics)
	}
}

func BenchmarkAsyncPublisher_PublishLog(b *testing.B) {
	// Setup
	logger := zerolog.Nop()
	p := NewAsyncPublisher(logger, nopObserver{})

	ctx := context.Background()
	now := time.Unix(1774200253, 0)
	ip := "127.0.0.1"

	metrics := make([]model.Metrics, 0, 100)
	for i := 0; i < 50; i++ {
		v := float64(i)
		d := int64(i)
		metrics = append(metrics, model.Metrics{ID: "m" + strconv.Itoa(i), MType: model.Gauge, Value: &v})
		metrics = append(metrics, model.Metrics{ID: "c" + strconv.Itoa(i), MType: model.Counter, Delta: &d})
	}
	b.ResetTimer()

	// Measure
	for b.Loop() {
		p.PublishLog(ctx, now, ip, metrics...)
	}
}
