package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zerogravity-82/metrics/internal/model"

	"github.com/rs/zerolog"
)

var (
	sinkInt    int
	sinkErr    error
	sinkMetric model.Metrics
)

type discardResponseWriter struct {
	header      http.Header
	statusCode  int
	wroteHeader bool
}

func newDiscardResponseWriter() *discardResponseWriter {
	return &discardResponseWriter{header: make(http.Header)}
}

func (w *discardResponseWriter) Header() http.Header {
	return w.header
}

func (w *discardResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return io.Discard.Write(p)
}

func (w *discardResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.statusCode = statusCode
	w.wroteHeader = true
}

type nopStorage struct{}

func (nopStorage) UpdateMetric(context.Context, model.Metrics) error    { return nil }
func (nopStorage) UpdateMetrics(context.Context, []model.Metrics) error { return nil }
func (nopStorage) GetMetric(context.Context, string, string) (model.Metrics, error) {
	return model.Metrics{}, nil
}
func (nopStorage) GetAll(context.Context) (map[string]model.Metrics, error) {
	return map[string]model.Metrics{}, nil
}
func (nopStorage) Ping(context.Context) error { return nil }
func (nopStorage) Close() error               { return nil }

type nopAuditPublisher struct{}

func (nopAuditPublisher) PublishLog(context.Context, time.Time, string, ...model.Metrics) {}

func BenchmarkSigningResponseWriter_CalculateSignature(b *testing.B) {
	// Setup
	inner := newDiscardResponseWriter()
	key := "secret"
	logger := zerolog.Nop()
	writer := newSigningResponseWriter(inner, key, logger)
	data := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	sinkInt, sinkErr = writer.Write(data)
	b.ResetTimer()

	// Measure
	for b.Loop() {
		writer.WriteHeader(http.StatusOK)
	}
}

func BenchmarkMetricRouter_validateSignature(b *testing.B) {
	// Setup
	body := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	key := "secret"
	sig := hex.EncodeToString(generateSignature(body, key))
	b.ResetTimer()

	// Measure
	for b.Loop() {
		sinkErr = validateSignature(sig, body, key)
	}
}

func BenchmarkMetricRouter_buildMetric(b *testing.B) {
	b.Run("gauge metric", func(b *testing.B) {
		b.ResetTimer()
		// Measure
		for b.Loop() {
			sinkMetric, sinkErr = buildMetric(model.Gauge, "RandomValue", "123.45")
		}
	})
	b.Run("counter metric", func(b *testing.B) {
		b.ResetTimer()
		// Measure
		for b.Loop() {
			sinkMetric, sinkErr = buildMetric(model.Counter, "PollCount", "777")
		}
	})
}

func BenchmarkMetricRouter_UpdateRoute(b *testing.B) {
	// Setup
	logger := zerolog.Nop()
	r := MetricRouter(nopStorage{}, nopAuditPublisher{}, "", logger)

	payload := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	b.ResetTimer()

	// Measure
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := newDiscardResponseWriter()
		r.ServeHTTP(w, req)
		sinkInt = w.statusCode
	}
}

func BenchmarkMetricRouter_UpdateRoute_GzipIn_GzipOut(b *testing.B) {
	// Setup
	logger := zerolog.Nop()
	r := MetricRouter(nopStorage{}, nopAuditPublisher{}, "", logger)

	payload := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	gzPayload := gzipBody(payload)
	b.ResetTimer()

	// Measure
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(gzPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		w := newDiscardResponseWriter()
		r.ServeHTTP(w, req)
		sinkInt = w.statusCode
	}
}

func gzipBody(payload []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	sinkInt, sinkErr = zw.Write(payload)
	sinkErr = zw.Close()
	return buf.Bytes()
}

func BenchmarkMetricRouter_UpdateRoute_WithSignature(b *testing.B) {
	// Setup
	logger := zerolog.Nop()
	key := "secret"
	r := MetricRouter(nopStorage{}, nopAuditPublisher{}, key, logger)

	payload := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	sig := hex.EncodeToString(generateSignature(payload, key))
	b.ResetTimer()

	// Measure
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HashSHA256", sig)

		w := newDiscardResponseWriter()
		r.ServeHTTP(w, req)
		sinkInt = w.statusCode
	}
}
