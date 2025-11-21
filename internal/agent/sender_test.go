package agent

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/model"
)

func TestPollMetrics(t *testing.T) {
	// Arrange
	m := metrics{}
	m.memStat = make(map[string]float64)

	// Act
	pollMetrics(&m)

	// Assert
	assert.Contains(t, m.memStat, "Alloc")
	assert.Contains(t, m.memStat, "BuckHashSys")
	assert.Contains(t, m.memStat, "Frees")
	assert.Contains(t, m.memStat, "GCCPUFraction")
	assert.Contains(t, m.memStat, "GCSys")
	assert.Contains(t, m.memStat, "HeapAlloc")
	assert.Contains(t, m.memStat, "HeapIdle")
	assert.Contains(t, m.memStat, "HeapInuse")
	assert.Contains(t, m.memStat, "HeapObjects")
	assert.Contains(t, m.memStat, "HeapReleased")
	assert.Contains(t, m.memStat, "HeapSys")
	assert.Contains(t, m.memStat, "LastGC")
	assert.Contains(t, m.memStat, "Lookups")
	assert.Contains(t, m.memStat, "MCacheInuse")
	assert.Contains(t, m.memStat, "MCacheSys")
	assert.Contains(t, m.memStat, "MSpanInuse")
	assert.Contains(t, m.memStat, "MSpanSys")
	assert.Contains(t, m.memStat, "Mallocs")
	assert.Contains(t, m.memStat, "NextGC")
	assert.Contains(t, m.memStat, "NumForcedGC")
	assert.Contains(t, m.memStat, "NumGC")
	assert.Contains(t, m.memStat, "OtherSys")
	assert.Contains(t, m.memStat, "PauseTotalNs")
	assert.Contains(t, m.memStat, "StackInuse")
	assert.Contains(t, m.memStat, "StackSys")
	assert.Contains(t, m.memStat, "Sys")
	assert.Contains(t, m.memStat, "TotalAlloc")
}

func TestSendReport(t *testing.T) {
	// Arrange
	metrics := metrics{}
	metrics.memStat = make(map[string]float64)
	pollMetrics(&metrics)

	var sentMetrics []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert
		assert.Equal(t, "/update", r.URL.Path)

		zr, err := gzip.NewReader(r.Body)
		require.NoError(t, err)

		var m model.Metrics
		dec := json.NewDecoder(zr)
		err = dec.Decode(&m)
		require.NoError(t, err)

		assert.NotContains(t, sentMetrics, m.ID) // Гарантирует, что каждая метрика отправлена не более одного раза
		sentMetrics = append(sentMetrics, m.ID)
		switch m.ID {
		case "PollCount":
			assert.Equal(t, model.Counter, m.MType)
			assert.Equal(t, metrics.pollCount, *m.Delta)
		case "RandomValue":
			assert.Equal(t, model.Gauge, m.MType)
			assert.Equal(t, metrics.randomValue, *m.Value)
		default:
			assert.Equal(t, model.Gauge, m.MType)
			assert.Equal(t, metrics.memStat[m.ID], *m.Value)
		}
	}))
	defer server.Close()
	httpClient := resty.New()

	// Act
	sendReport(server.URL, &metrics, httpClient)

	// Assert
	assert.Equal(t, len(metrics.memStat)+2, len(sentMetrics))
}

func TestAddDefaultSchema(t *testing.T) {
	// Arrange
	testTable := []struct {
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
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			resultURL := addDefaultURLSchema(tt.inputURL)

			// Assert
			assert.Equal(t, tt.wantResultURL, resultURL)
		})
	}
}
