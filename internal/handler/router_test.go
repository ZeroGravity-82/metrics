package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/service"
)

func TestUpdateMetricHandler(t *testing.T) {
	ms := service.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms))
	defer ts.Close()

	tests := []struct {
		name           string
		method         string
		contentType    string
		mType          string
		mName          string
		mValue         string
		wantStatusCode int
	}{
		{
			name:           "fail when method is not allowed",
			method:         http.MethodPut,
			contentType:    "text/plain",
			mType:          "gauge",
			mName:          "RandomValue",
			mValue:         "12345",
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:           "fail when metric name is missing",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "gauge",
			mName:          "",
			mValue:         "12345",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "fail when counter delta value is invalid",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "counter",
			mName:          "PollCount",
			mValue:         "5abc",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail when gauge value is invalid",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "gauge",
			mName:          "GCCPUFraction",
			mValue:         "0.5abc",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail with unsupported metric type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "unsupported",
			mName:          "GCCPUFraction",
			mValue:         "0.5",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "can update counter metric",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "counter",
			mName:          "PollCount",
			mValue:         "15",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "can update gauge metric",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "gauge",
			mName:          "GCCPUFraction",
			mValue:         "0.5",
			wantStatusCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			target := fmt.Sprintf("/update/%s/%s/%s", tt.mType, tt.mName, tt.mValue)
			req, err := http.NewRequest(tt.method, ts.URL+target, http.NoBody)
			require.NoError(t, err)
			req.Header.Set("Content-Type", tt.contentType)

			// Act
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Assert
			_, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
		})
	}
}

func TestGetMetricHandler(t *testing.T) {
	ms := service.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms))
	defer ts.Close()

	tests := []struct {
		name           string
		method         string
		mType          string
		mName          string
		wantStatusCode int
		wantValue      string
	}{
		{
			name:           "fail when method is not allowed",
			method:         http.MethodPut,
			mType:          "gauge",
			mName:          "RandomValue",
			wantStatusCode: http.StatusMethodNotAllowed,
			wantValue:      "",
		},
		{
			name:           "fail when counter metric not found",
			method:         http.MethodGet,
			mType:          "counter",
			mName:          "UnknownCounterMetric",
			wantStatusCode: http.StatusNotFound,
			wantValue:      "",
		},
		{
			name:           "fail when gauge metric not found",
			method:         http.MethodGet,
			mType:          "gauge",
			mName:          "UnknownGaugeMetric",
			wantStatusCode: http.StatusNotFound,
			wantValue:      "",
		},
		{
			name:           "can get counter metric",
			method:         http.MethodGet,
			mType:          "counter",
			mName:          "PollCount",
			wantStatusCode: http.StatusOK,
			wantValue:      "777",
		},
		{
			name:           "can get gauge metric",
			method:         http.MethodGet,
			mType:          "gauge",
			mName:          "RandomValue",
			wantStatusCode: http.StatusOK,
			wantValue:      "12345",
		},
	}
	// Arrange
	_ = ms.UpdateMetric(model.Counter, "PollCount", "777")
	_ = ms.UpdateMetric(model.Gauge, "RandomValue", "12345")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			target := fmt.Sprintf("/value/%s/%s", tt.mType, tt.mName)
			req, err := http.NewRequest(tt.method, ts.URL+target, http.NoBody)
			require.NoError(t, err)

			// Act
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Assert
			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
			if tt.wantValue != "" {
				assert.Equal(t, tt.wantValue, string(bodyBytes))
			}
		})
	}
}

func TestGetMetricListHandler(t *testing.T) {
	ms := service.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms))
	defer ts.Close()

	tests := []struct {
		name           string
		method         string
		wantStatusCode int
	}{
		{
			name:           "fail when method is not allowed",
			method:         http.MethodPut,
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:           "can get metric list",
			method:         http.MethodGet,
			wantStatusCode: http.StatusOK,
		},
	}
	// Arrange
	_ = ms.UpdateMetric(model.Gauge, "Alloc", "310552")
	_ = ms.UpdateMetric(model.Gauge, "BuckHashSys", "3342")
	_ = ms.UpdateMetric(model.Gauge, "OtherSys", "606658")
	_ = ms.UpdateMetric(model.Counter, "PollCount", "777777777777777")
	_ = ms.UpdateMetric(model.Gauge, "RandomValue", "3685246675")
	_ = ms.UpdateMetric(model.Gauge, "GCCPUFraction", "0.00000012345678912345")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			req, err := http.NewRequest(tt.method, ts.URL, http.NoBody)
			require.NoError(t, err)

			// Act
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Assert
			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
			bodyString := string(bodyBytes)
			if resp.StatusCode == http.StatusOK {
				assert.Contains(t, bodyString, "Alloc: 310552")
				assert.Contains(t, bodyString, "BuckHashSys: 3342")
				assert.Contains(t, bodyString, "OtherSys: 606658")
				assert.Contains(t, bodyString, "PollCount: 777777777777777")
				assert.Contains(t, bodyString, "RandomValue: 3.685246675e+09")
				assert.Contains(t, bodyString, "GCCPUFraction: 1.2345678912345e-07")
			}
		})
	}
}
