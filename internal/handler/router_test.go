package handler

import (
	"bytes"
	"compress/gzip"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/repository"
)

func int64Pointer(v int64) *int64 {
	return &v
}

func float64Pointer(v float64) *float64 {
	return &v
}

func TestUpdateMetricHandler(t *testing.T) {
	// Arrange
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"server"}

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ms := repository.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms, logger, db))
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
			name:           "fail to update counter with invalid metric type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "gauge",
			mName:          "PollCount",
			mValue:         "15",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail to update gauge with invalid metric type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "counter",
			mName:          "RandomValue",
			mValue:         "0.7",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "can add new gauge metric",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "gauge",
			mName:          "GCCPUFraction",
			mValue:         "0.5",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "can add new counter metric",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "counter",
			mName:          "MyCounter",
			mValue:         "5",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "can update existed gauge metric",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "gauge",
			mName:          "RandomValue",
			mValue:         "23456",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "can update existed counter metric",
			method:         http.MethodPost,
			contentType:    "text/plain",
			mType:          "counter",
			mName:          "PollCount",
			mValue:         "15",
			wantStatusCode: http.StatusOK,
		},
	}
	var v1 int64 = 777
	m1 := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &v1}
	_ = ms.UpdateMetric(m1)
	var v2 float64 = 12345
	m2 := model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &v2}
	_ = ms.UpdateMetric(m2)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			URL, err := url.JoinPath(ts.URL, "/update", tt.mType, tt.mName, tt.mValue)
			require.NoError(t, err)
			req, err := http.NewRequest(tt.method, URL, http.NoBody)
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

func TestUpdateHandler(t *testing.T) {
	// Arrange
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"server"}

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ms := repository.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms, logger, db))
	defer ts.Close()

	tests := []struct {
		name           string
		method         string
		contentType    string
		body           string
		wantStatusCode int
	}{
		{
			name:           "fail when unsupported content type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			body:           `{"id":"RandomValue","type":"gauge","value":12345}`,
			wantStatusCode: http.StatusUnsupportedMediaType,
		},
		{
			name:           "fail when method is not allowed",
			method:         http.MethodPut,
			contentType:    "application/json",
			body:           `{"id":"RandomValue","type":"gauge","value":12345}`,
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:           "fail when metric name is missing",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"","type":"gauge","value":12345}`,
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "fail when counter delta value is invalid",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"PollCount","type":"counter","delta":"foo"}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail when gauge value is invalid",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"GCCPUFraction","type":"gauge","value":"bar"}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail with unsupported metric type",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"GCCPUFraction","type":"unsupported","value":0.5}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail to add counter with invalid metric type",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"SomeCounter","type":"gauge","delta":15}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail to add gauge with invalid metric type",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"SomeGauge","type":"counter","value":0.5}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail to update counter with invalid metric type",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"PollCount","type":"gauge","delta":15}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail to update gauge with invalid metric type",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"RandomValue","type":"counter","value":0.7}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail to update counter without delta",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"PollCount","type":"counter"}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "fail to update gauge without value",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"GCCPUFraction","type":"gauge"}`,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "can add new gauge metric",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"GCCPUFraction","type":"gauge","value":0.5}`,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "can add new counter metric",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"MyCounter","type":"counter","delta":5}`,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "can update existed gauge metric",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"RandomValue","type":"gauge","value":23456}`,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "can update existed counter metric",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"id":"PollCount","type":"counter","delta":15}`,
			wantStatusCode: http.StatusOK,
		},
	}
	_ = ms.UpdateMetric(model.Metrics{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(777)})
	_ = ms.UpdateMetric(model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: float64Pointer(12345)})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			buf, err := compressWithGzip(tt.body)
			require.NoError(t, err)
			URL, err := url.JoinPath(ts.URL, "/update")
			require.NoError(t, err)
			req, err := http.NewRequest(tt.method, URL, buf)
			require.NoError(t, err)
			req.Header.Set("Content-Type", tt.contentType)
			req.Header.Set("Content-Encoding", "gzip")

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

func compressWithGzip(body string) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)
	zb := gzip.NewWriter(buf)
	if _, err := zb.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := zb.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

func TestGetMetricHandler(t *testing.T) {
	// Arrange
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"server"}

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ms := repository.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms, logger, db))
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
			name:           "fail with unsupported metric type",
			method:         http.MethodGet,
			mType:          "unsupported",
			mName:          "PollCount",
			wantStatusCode: http.StatusBadRequest,
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
	_ = ms.UpdateMetric(model.Metrics{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(777)})
	_ = ms.UpdateMetric(model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: float64Pointer(12345)})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			URL, err := url.JoinPath(ts.URL, "/value", tt.mType, tt.mName)
			require.NoError(t, err)
			req, err := http.NewRequest(tt.method, URL, http.NoBody)
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

func TestGetHandler(t *testing.T) {
	// Arrange
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"server"}

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ms := repository.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms, logger, db))
	defer ts.Close()

	tests := []struct {
		name            string
		method          string
		contentType     string
		body            string
		wantStatusCode  int
		wantValue       string
		wantContentType string
	}{
		{
			name:            "fail when unsupported content type",
			method:          http.MethodPost,
			contentType:     "text/plain",
			body:            `{"id":"RandomValue","type":"gauge"}`,
			wantStatusCode:  http.StatusUnsupportedMediaType,
			wantValue:       "",
			wantContentType: "",
		},
		{
			name:            "fail when method is not allowed",
			method:          http.MethodGet,
			contentType:     "application/json",
			body:            `{"id":"RandomValue","type":"gauge"}`,
			wantStatusCode:  http.StatusMethodNotAllowed,
			wantValue:       "",
			wantContentType: "",
		},
		{
			name:            "fail with invalid JSON in request body",
			method:          http.MethodPost,
			contentType:     "application/json",
			body:            ``,
			wantStatusCode:  http.StatusBadRequest,
			wantValue:       "",
			wantContentType: "text/plain; charset=utf-8",
		},
		{
			name:            "fail when counter metric not found",
			method:          http.MethodPost,
			contentType:     "application/json",
			body:            `{"id":"UnknownCounterMetric","type":"counter"}`,
			wantStatusCode:  http.StatusNotFound,
			wantValue:       "",
			wantContentType: "text/plain; charset=utf-8",
		},
		{
			name:            "fail when gauge metric not found",
			method:          http.MethodPost,
			contentType:     "application/json",
			body:            `{"id":"UnknownGaugeMetric","type":"gauge"}`,
			wantStatusCode:  http.StatusNotFound,
			wantValue:       "",
			wantContentType: "text/plain; charset=utf-8",
		},
		{
			name:            "fail with unsupported metric type",
			method:          http.MethodPost,
			contentType:     "application/json",
			body:            `{"id":"PollCount","type":"unsupported"}`,
			wantStatusCode:  http.StatusBadRequest,
			wantValue:       "",
			wantContentType: "text/plain; charset=utf-8",
		},
		{
			name:            "can get counter metric",
			method:          http.MethodPost,
			contentType:     "application/json",
			body:            `{"id":"PollCount","type":"counter"}`,
			wantStatusCode:  http.StatusOK,
			wantValue:       `{"id":"PollCount","type":"counter","delta":777}`,
			wantContentType: "application/json",
		},
		{
			name:            "can get gauge metric",
			method:          http.MethodPost,
			contentType:     "application/json",
			body:            `{"id":"RandomValue","type":"gauge"}`,
			wantStatusCode:  http.StatusOK,
			wantValue:       `{"id":"RandomValue","type":"gauge","value":12345}`,
			wantContentType: "application/json",
		},
	}
	_ = ms.UpdateMetric(model.Metrics{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(777)})
	_ = ms.UpdateMetric(model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: float64Pointer(12345)})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			buf, err := compressWithGzip(tt.body)
			require.NoError(t, err)
			URL, err := url.JoinPath(ts.URL, "/value")
			require.NoError(t, err)
			req, err := http.NewRequest(tt.method, URL, buf)
			require.NoError(t, err)
			req.Header.Set("Content-Type", tt.contentType)
			req.Header.Set("Content-Encoding", "gzip")

			// Act
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Assert
			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
			if tt.wantValue != "" {
				assert.JSONEq(t, tt.wantValue, string(bodyBytes))
			}
			assert.Equal(t, tt.wantContentType, resp.Header.Get("Content-Type"))
		})
	}
}

func TestGetMetricListHandler(t *testing.T) {
	// Arrange
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"server"}

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ms := repository.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms, logger, db))
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
	_ = ms.UpdateMetric(model.Metrics{ID: "Alloc", MType: model.Gauge, Value: float64Pointer(310552)})
	_ = ms.UpdateMetric(model.Metrics{ID: "BuckHashSys", MType: model.Gauge, Value: float64Pointer(3342)})
	_ = ms.UpdateMetric(model.Metrics{ID: "OtherSys", MType: model.Gauge, Value: float64Pointer(606658)})
	_ = ms.UpdateMetric(model.Metrics{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(777777777777777)})
	_ = ms.UpdateMetric(model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: float64Pointer(3685246675)})
	_ = ms.UpdateMetric(model.Metrics{ID: "GCCPUFraction", MType: model.Gauge, Value: float64Pointer(0.00000012345678912345)})
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
				assert.Contains(t, bodyString, "RandomValue: 3.685246675e&#43;09")
				assert.Contains(t, bodyString, "GCCPUFraction: 1.2345678912345e-07")
			}
		})
	}
}

func TestPingHandler(t *testing.T) {
	// Arrange
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"server"}

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	ms := repository.NewMemStorage()
	ts := httptest.NewServer(MetricRouter(ms, logger, db))
	defer ts.Close()

	tests := []struct {
		name                 string
		forceCloseConnection bool
		wantStatusCode       int
	}{
		{
			name:                 "successful ping",
			forceCloseConnection: false,
			wantStatusCode:       http.StatusOK,
		},
		{
			name:                 "database connection failed",
			forceCloseConnection: true,
			wantStatusCode:       http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			URL, err := url.JoinPath(ts.URL, "/ping")
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodGet, URL, http.NoBody)
			require.NoError(t, err)
			mock.ExpectPing()
			if tt.forceCloseConnection {
				_ = db.Close()
			}

			// Act
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Assert
			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
			assert.Empty(t, bodyBytes)
			if !tt.forceCloseConnection {
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			}
		})
	}
}

//
//func TestStoreMetric(t *testing.T) {
//	// Arrange
//	metrics := make(map[string]model.Metrics)
//	metrics["LastGC"] = model.Metrics{
//		ID:    "LastGC",
//		MType: "gauge",
//		Value: float64Pointer(1257894000000000000),
//	}
//	metrics["NumGC"] = model.Metrics{
//		ID:    "NumGC",
//		MType: "counter",
//		Delta: int64Pointer(42),
//	}
//	tmpFile, err := os.CreateTemp("", "metrics*.json")
//	require.NoError(t, err)
//	defer os.Remove(tmpFile.Name())
//
//	// Act
//	err = storeMetrics(metrics, tmpFile.Name())
//	require.NoError(t, err)
//
//	// Assert
//	data, err := os.ReadFile(tmpFile.Name())
//	require.NoError(t, err)
//
//	var actualMetricSlice []model.Metrics
//	err = json.Unmarshal(data, &actualMetricSlice)
//	require.NoError(t, err)
//
//	expectedJSON := `[{"id":"LastGC","type":"gauge","value":1257894000000000000},{"id":"NumGC","type":"counter","delta":42}]`
//	var expectedMetricSlice []model.Metrics
//	err = json.Unmarshal([]byte(expectedJSON), &expectedMetricSlice)
//	require.NoError(t, err)
//
//	assert.Equal(t, expectedMetricSlice, actualMetricSlice)
//}
