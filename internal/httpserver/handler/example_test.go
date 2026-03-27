package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/repository"
	"zerogravity-82/metrics/internal/service/audit"
)

func gzipJSON(v any) (*bytes.Buffer, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

// ExampleMetricRouter_updateValueText показывает работу с ручкой `POST /update/{type}/{name}/{value}`.
func ExampleMetricRouter_updateValueText() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)

	ts := httptest.NewServer(MetricRouter(store, auditPublisher, "", logger))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/update/gauge/RandomValue/123.45", http.NoBody)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := ts.Client().Do(req)
	if err != nil {
		fmt.Println("request error")
		return
	}
	_ = resp.Body.Close()
	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}

// ExampleMetricRouter_getValueText показывает работу с ручкой `GET /value/{type}/{name}`.
func ExampleMetricRouter_getValueText() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	_ = store.UpdateMetric(
		context.Background(),
		model.Metrics{ID: "PollCount", MType: model.Counter, Delta: func() *int64 { v := int64(777); return &v }()},
	)

	ts := httptest.NewServer(MetricRouter(store, auditPublisher, "", logger))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/value/counter/PollCount", http.NoBody)
	resp, _ := ts.Client().Do(req)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	fmt.Printf("%d %s\n", resp.StatusCode, string(body))

	// Output:
	// 200 777
}

// ExampleMetricRouter_updateValueJSON показывает работу с ручкой `POST /update`
func ExampleMetricRouter_updateValueJSON() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)

	ts := httptest.NewServer(MetricRouter(store, auditPublisher, "", logger))
	defer ts.Close()

	payload := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: func() *int64 { v := int64(5); return &v }()}
	buf, _ := gzipJSON(payload)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/update", buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, _ := ts.Client().Do(req)
	_ = resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}

// ExampleMetricRouter_updatesValuesJSON показывает работу с ручкой `POST /updates`
func ExampleMetricRouter_updatesValuesJSON() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)

	ts := httptest.NewServer(MetricRouter(store, auditPublisher, "", logger))
	defer ts.Close()

	payload := []model.Metrics{
		{ID: "GCCPUFraction", MType: model.Gauge, Value: func() *float64 { v := 0.5; return &v }()},
		{ID: "MyCounter", MType: model.Counter, Delta: func() *int64 { v := int64(10); return &v }()},
	}
	buf, _ := gzipJSON(payload)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/updates", buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, _ := ts.Client().Do(req)
	_ = resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}

// ExampleMetricRouter_getValueJSON показывает работу с ручкой `POST /value`
func ExampleMetricRouter_getValueJSON() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	_ = store.UpdateMetric(
		context.Background(),
		model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: func() *float64 { v := 123.45; return &v }()},
	)

	ts := httptest.NewServer(MetricRouter(store, auditPublisher, "", logger))
	defer ts.Close()

	payload := model.Metrics{ID: "RandomValue", MType: model.Gauge}
	buf, _ := gzipJSON(payload)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/value", buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, _ := ts.Client().Do(req)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	fmt.Printf("%d %s\n", resp.StatusCode, string(bytes.TrimSpace(body)))

	// Output:
	// 200 {"id":"RandomValue","type":"gauge","value":123.45}
}

// ExampleMetricRouter_ping показывает работу с ручкой `GET /ping`
func ExampleMetricRouter_ping() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)

	ts := httptest.NewServer(MetricRouter(store, auditPublisher, "", logger))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/ping", http.NoBody)
	resp, _ := ts.Client().Do(req)
	_ = resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}
