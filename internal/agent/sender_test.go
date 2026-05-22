package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/model"
)

func int64Pointer(v int64) *int64 {
	return &v
}

func float64Pointer(v float64) *float64 {
	return &v
}

// TestPollMetrics проверяет, что pollMetrics() собирает райнтайм-метрики и счетчик опросов.
func TestPollMetrics(t *testing.T) {
	// Arrange
	m := newMetrics()

	// Act
	m.pollMetrics()

	// Assert
	assert.Contains(t, m.data, "Alloc")
	assert.Contains(t, m.data, "BuckHashSys")
	assert.Contains(t, m.data, "Frees")
	assert.Contains(t, m.data, "GCCPUFraction")
	assert.Contains(t, m.data, "GCSys")
	assert.Contains(t, m.data, "HeapAlloc")
	assert.Contains(t, m.data, "HeapIdle")
	assert.Contains(t, m.data, "HeapInuse")
	assert.Contains(t, m.data, "HeapObjects")
	assert.Contains(t, m.data, "HeapReleased")
	assert.Contains(t, m.data, "HeapSys")
	assert.Contains(t, m.data, "LastGC")
	assert.Contains(t, m.data, "Lookups")
	assert.Contains(t, m.data, "MCacheInuse")
	assert.Contains(t, m.data, "MCacheSys")
	assert.Contains(t, m.data, "MSpanInuse")
	assert.Contains(t, m.data, "MSpanSys")
	assert.Contains(t, m.data, "Mallocs")
	assert.Contains(t, m.data, "NextGC")
	assert.Contains(t, m.data, "NumForcedGC")
	assert.Contains(t, m.data, "NumGC")
	assert.Contains(t, m.data, "OtherSys")
	assert.Contains(t, m.data, "PauseTotalNs")
	assert.Contains(t, m.data, "StackInuse")
	assert.Contains(t, m.data, "StackSys")
	assert.Contains(t, m.data, "Sys")
	assert.Contains(t, m.data, "TotalAlloc")
	assert.Contains(t, m.data, "RandomValue")
}

// TestPollUtilMetrics проверяет, что pollUtilMetrics() собирает метрики памяти и CPU.
func TestPollUtilMetrics(t *testing.T) {
	// Arrange
	logger := zerolog.Nop()
	m := newMetrics()

	// Act
	m.pollUtilMetrics(logger)

	// Assert
	assert.Contains(t, m.data, "TotalMemory")
	assert.Contains(t, m.data, "FreeMemory")
	assert.Contains(t, m.data, "CPUutilization1")
}

// TestResetPollCount проверяет, что resetPollCount() сбрасывает счетчик PollCount.
func TestResetPollCount(t *testing.T) {
	// Arrange
	m := newMetrics()

	// Act
	m.resetPollCount()

	// Assert
	assert.Equal(
		t,
		model.Metrics{ID: "PollCount", MType: model.Counter, Value: nil, Delta: int64Pointer(0)},
		m.data["PollCount"],
	)
}

// TestIncrementPollCount проверяет, что incrementPollCount() увеличивает PollCount на единицу.
func TestIncrementPollCount(t *testing.T) {
	// Arrange
	m := newMetrics()
	m.resetPollCount()

	// Act
	incrementPollCount(m)

	// Assert
	assert.Equal(
		t,
		model.Metrics{ID: "PollCount", MType: model.Counter, Value: nil, Delta: int64Pointer(1)},
		m.data["PollCount"],
	)
}

// TestCopyMetricsAndResetPollCount проверяет, что copyMetricsAndResetPollCount() возвращает копию и обнуляет PollCount
// в исходных данных.
func TestCopyMetricsAndResetPollCount(t *testing.T) {
	// Arrange
	pollCount := int64(10)
	originalMetrics := &metrics{
		data: map[string]model.Metrics{
			"Alloc":     {ID: "Alloc", MType: model.Gauge, Value: float64Pointer(12345)},
			"PollCount": {ID: "PollCount", MType: model.Counter, Delta: int64Pointer(pollCount)},
			"HeapAlloc": {ID: "HeapAlloc", MType: model.Gauge, Value: float64Pointer(67890)},
		},
	}

	// Act
	copiedMetrics := originalMetrics.copyMetricsAndResetPollCount()

	// Assert
	assert.Equal(t, len(originalMetrics.data), len(copiedMetrics))
	for key, originalMetric := range originalMetrics.data {
		copiedMetric, exists := copiedMetrics[key]
		assert.True(t, exists)
		if key != "PollCount" {
			assert.Equal(t, originalMetric, copiedMetric)
		}
	}
	assert.Equal(t, pollCount, *copiedMetrics["PollCount"].Delta) // В копии сохранилось значение счетчика PollCount,
	assert.Zero(t, *originalMetrics.data["PollCount"].Delta)      // но в оригинале оно сбросилось

	copiedMetrics["Alloc"] = model.Metrics{ID: "Alloc", MType: model.Gauge, Value: float64Pointer(54321)}
	assert.NotEqual(t, originalMetrics.data["Alloc"], copiedMetrics["Alloc"]) // Изменение копии не влияет на оригинал
}

// TestSendReport проверяет, что sendReport() отправляет метрики на endpoint `/updates` и при необходимости подписывает
// запрос.
func TestSendReport(t *testing.T) {
	// Arrange
	sentMetrics := newMetrics()
	sentMetrics.pollMetrics()

	tests := []struct {
		name                string
		signatureKey        string
		wantSignatureHeader bool
	}{
		{
			name:                "with signature",
			signatureKey:        "secret",
			wantSignatureHeader: true,
		},
		{
			name:                "without signature",
			signatureKey:        "",
			wantSignatureHeader: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var processedMetricIDs []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Arrange
				zr, err := gzip.NewReader(r.Body)
				require.NoError(t, err)
				jsonBz, err := io.ReadAll(zr)
				require.NoError(t, err)

				var receivedMetrics []model.Metrics
				jr := bytes.NewReader(jsonBz)
				dec := json.NewDecoder(jr)
				err = dec.Decode(&receivedMetrics)
				require.NoError(t, err)

				// Assert
				assert.Equal(t, "/updates", r.URL.Path)

				if tt.wantSignatureHeader {
					var decodedSig []byte
					decodedSig, err = hex.DecodeString(r.Header.Get("HashSHA256"))
					require.NoError(t, err)
					assert.True(t, hmac.Equal(generateSignature(jsonBz, tt.signatureKey), decodedSig))
				} else {
					assert.Empty(t, r.Header.Get("HashSHA256"))
				}
				assert.NotEmpty(t, r.Header.Get("X-Real-IP"))
				assert.NotNil(t, net.ParseIP(r.Header.Get("X-Real-IP")))

				for _, m := range receivedMetrics {
					assert.NotContains(t, processedMetricIDs, m.ID) // Гарантирует, что каждая метрика отправлена не более одного раза
					processedMetricIDs = append(processedMetricIDs, m.ID)
					assert.Equal(t, sentMetrics.data[m.ID], m)
				}
			}))
			defer server.Close()
			httpClient := resty.New()

			// Act
			err := sendReport(server.URL, tt.signatureKey, "", sentMetrics.data, httpClient)
			require.NoError(t, err)

			// Assert
			assert.Equal(t, len(sentMetrics.data), len(processedMetricIDs))
		})
	}
}

func generateSignature(data []byte, key string) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return h.Sum(nil)
}

// TestOutboundIPFor проверяет, что outboundIPFor() определяет локальный IP.
func Test_outboundIPFor(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	// Act
	ip, err := outboundIPFor(server.URL)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, ip)
	assert.NotNil(t, net.ParseIP(ip))
}

// TestOutboundIPFor_InvalidURL проверяет, что outboundIPFor() возвращает ошибку для некорректного URL.
func Test_outboundIPFor_InvalidURL(t *testing.T) {
	// Arrange
	target := "://bad-url"

	// Act
	ip, err := outboundIPFor(target)

	// Assert
	require.Error(t, err)
	assert.Empty(t, ip)
}

// TestAddDefaultSchema проверяет, что addDefaultURLSchema() подставляет корректную схему по умолчанию.
func TestAddDefaultSchema(t *testing.T) {
	// Arrange
	tests := []struct {
		name          string
		inputURL      string
		wantResultURL string
	}{
		{
			name:          "localhost",
			inputURL:      "localhost:8081",
			wantResultURL: "http://localhost:8081",
		},
		{
			name:          "port only",
			inputURL:      ":8081",
			wantResultURL: "http://localhost:8081",
		},
		{
			name:          "regular IP address",
			inputURL:      "192.168.1.101:8081",
			wantResultURL: "https://192.168.1.101:8081",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			resultURL := addDefaultURLSchema(tt.inputURL)

			// Assert
			assert.Equal(t, tt.wantResultURL, resultURL)
		})
	}
}

// TestRetryAfterFunc проверяет последовательность задержек, которую retryAfterFunc() задает для повторных попыток
// resty.
func TestRetryAfterFunc(t *testing.T) {
	// Arrange
	httpClient := resty.New()
	response := &resty.Response{}

	retryAfter := retryAfterFunc()

	// Act (first retry)
	duration, err := retryAfter(httpClient, response)

	// Assert (first retry)
	require.NoError(t, err)
	assert.Equal(t, 1*time.Second, duration)

	// Act (second retry)
	duration, err = retryAfter(httpClient, response)

	// Assert (second retry)
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, duration)

	// Act (subsequent retries)
	duration, err = retryAfter(httpClient, response)

	// Assert (subsequent retries)
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, duration)
}

// TestRestorePollCount проверяет, что restorePollCount() возвращает значение PollCount после неуспешной отправки.
func TestRestorePollCount(t *testing.T) {
	// Arrange
	sentMetrics := newMetrics()

	// Act
	sentMetrics.restorePollCount(3)

	// Assert
	assert.Equal(t, int64(3), *sentMetrics.data["PollCount"].Delta)
}

// TestRun_GracefulShutdown_FlushesPendingMetrics проверяет, что при остановке Run() отправляет накопленные, но еще не
// зарепорченные метрики.
func TestRun_GracefulShutdown_FlushesPendingMetrics(t *testing.T) {
	// Arrange
	var requestCount atomic.Int32
	receivedMetricsCh := make(chan []model.Metrics, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		zr, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		defer zr.Close()

		jsonBz, err := io.ReadAll(zr)
		require.NoError(t, err)

		var receivedMetrics []model.Metrics
		err = json.Unmarshal(jsonBz, &receivedMetrics)
		require.NoError(t, err)

		receivedMetricsCh <- receivedMetrics
	}))
	defer server.Close()

	cfg := mustAgentConfigForTest(t, server.URL, "1h", "50ms", 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		Run(ctx, cfg, zerolog.Nop())
	}()

	time.Sleep(200 * time.Millisecond) // Даем агенту время накопить метрики, но не дойти до штатного ReportInterval.
	assert.Zero(t, requestCount.Load())

	cancel()

	// Assert
	select {
	case <-runDone:
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not stop after context cancellation")
	}

	select {
	case metrics := <-receivedMetricsCh:
		assert.NotEmpty(t, metrics)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for shutdown flush")
	}
	assert.EqualValues(t, 1, requestCount.Load())
}

func mustAgentConfigForTest(
	t *testing.T,
	serverAddr,
	reportInterval,
	pollInterval string,
	rateLimit int,
) config.AgentConfig {
	t.Helper() // нужен, чтобы место ошибки require.NoError отображалось в тестовых функциях, а не в этом хелпере.

	cfgJSON := []byte(`
{
  "address":"` + serverAddr + `",
  "report_interval":"` + reportInterval + `",
  "poll_interval":"` + pollInterval + `",
  "rate_limit":` + "1" + `
}
`)
	var cfg config.AgentConfig
	require.NoError(t, json.Unmarshal(cfgJSON, &cfg))
	cfg.RateLimit = rateLimit
	return cfg
}

// TestRun_GracefulShutdown_WaitsForInFlightSend проверяет, что Run() не завершается, пока не закончится уже начатая
// отправка.
func TestRun_GracefulShutdown_WaitsForInFlightSend(t *testing.T) {
	// Arrange
	requestStarted := make(chan struct{})
	releaseResponse := make(chan struct{})
	var requestStartedOnce atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestStartedOnce.CompareAndSwap(false, true) {
			close(requestStarted)
		}
		<-releaseResponse
	}))
	defer server.Close()

	cfg := mustAgentConfigForTest(t, server.URL, "50ms", "10ms", 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		Run(ctx, cfg, zerolog.Nop())
	}()

	select {
	case <-requestStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for request start")
	}

	cancel()

	select {
	case <-runDone:
		t.Fatal("agent stopped before in-flight request completed")
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseResponse)

	// Assert
	select {
	case <-runDone:
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not wait for in-flight request")
	}
}
