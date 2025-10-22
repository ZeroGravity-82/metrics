package agent

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"zerogravity-82/metrics/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestPollMetrics(t *testing.T) {
	// Arrange
	m := metrics{}
	m.memStat = make(map[string]any)

	// Act
	pollMetrics(&m)

	// Assert
	assert.IsType(t, uint64(0), m.memStat["Alloc"])
	assert.IsType(t, uint64(0), m.memStat["BuckHashSys"])
	assert.IsType(t, uint64(0), m.memStat["Frees"])
	assert.IsType(t, float64(0), m.memStat["GCCPUFraction"])
	assert.IsType(t, uint64(0), m.memStat["GCSys"])
	assert.IsType(t, uint64(0), m.memStat["HeapAlloc"])
	assert.IsType(t, uint64(0), m.memStat["HeapIdle"])
	assert.IsType(t, uint64(0), m.memStat["HeapInuse"])
	assert.IsType(t, uint64(0), m.memStat["HeapObjects"])
	assert.IsType(t, uint64(0), m.memStat["HeapReleased"])
	assert.IsType(t, uint64(0), m.memStat["HeapSys"])
	assert.IsType(t, uint64(0), m.memStat["LastGC"])
	assert.IsType(t, uint64(0), m.memStat["Lookups"])
	assert.IsType(t, uint64(0), m.memStat["MCacheInuse"])
	assert.IsType(t, uint64(0), m.memStat["MCacheSys"])
	assert.IsType(t, uint64(0), m.memStat["MSpanInuse"])
	assert.IsType(t, uint64(0), m.memStat["MSpanSys"])
	assert.IsType(t, uint64(0), m.memStat["Mallocs"])
	assert.IsType(t, uint64(0), m.memStat["NextGC"])
	assert.IsType(t, uint32(0), m.memStat["NumForcedGC"])
	assert.IsType(t, uint32(0), m.memStat["NumGC"])
	assert.IsType(t, uint64(0), m.memStat["OtherSys"])
	assert.IsType(t, uint64(0), m.memStat["PauseTotalNs"])
	assert.IsType(t, uint64(0), m.memStat["StackInuse"])
	assert.IsType(t, uint64(0), m.memStat["StackSys"])
	assert.IsType(t, uint64(0), m.memStat["Sys"])
	assert.IsType(t, uint64(0), m.memStat["TotalAlloc"])
	assert.Equal(t, uint64(1), m.pollCount)
	assert.IsType(t, uint32(0), m.randomValue)
}

func TestSendReport(t *testing.T) {
	// Arrange
	m := metrics{}
	m.memStat = make(map[string]any)
	pollMetrics(&m)

	sentMetricsCnt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert
		pathParts := strings.Split(r.URL.Path, "/")
		require.Len(t, pathParts, 5)
		method := pathParts[1]
		mType := pathParts[2]
		mName := pathParts[3]
		mValue := pathParts[4]

		assert.Equal(t, "update", method)
		switch mName {
		case "PollCount":
			assert.Equal(t, model.Counter, mType)
			assert.Equal(t, toString(m.pollCount), mValue)
		case "RandomValue":
			assert.Equal(t, model.Gauge, mType)
			assert.Equal(t, toString(m.randomValue), mValue)
		default:
			assert.Equal(t, model.Gauge, mType)
			assert.Equal(t, toString(m.memStat[mName]), mValue)
			assert.NotNil(t, m.memStat[mName])
			m.memStat[mName] = nil // Гарантирует, что каждая метрика отправлена не более одного раза
		}
		sentMetricsCnt++
	}))
	defer server.Close()

	// Act
	sendReport(server.URL, &m)

	// Assert
	assert.Equal(t, len(m.memStat)+2, sentMetricsCnt)
}

func toString(v any) string {
	return fmt.Sprintf("%v", v)
}
