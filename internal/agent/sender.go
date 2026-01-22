package agent

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/model"
)

type metrics struct {
	sync.Mutex
	data map[string]model.Metrics
}

func newMetrics() *metrics {
	m := metrics{
		data: make(map[string]model.Metrics),
	}
	m.resetPollCount()
	return &m
}

func (m *metrics) resetPollCount() {
	var v int64
	m.data["PollCount"] = model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &v}
}

func (m *metrics) addPollCount(delta int64) {
	v := *m.data["PollCount"].Delta
	v += delta
	m.data["PollCount"] = model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &v}
}

func (m *metrics) incrementPollCount() {
	v := *m.data["PollCount"].Delta
	v++
	m.data["PollCount"] = model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &v}
}

func Run(cfg config.AgentConfig, logger zerolog.Logger) {
	m := newMetrics()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Duration(cfg.PollInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.Lock()
			pollMetrics(m)
			m.incrementPollCount()
			m.Unlock()
		}
	}()
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Duration(cfg.PollInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.Lock()
			pollUtilMetrics(m, logger)
			m.Unlock()
		}
	}()
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Duration(cfg.ReportInterval) * time.Second)
		defer ticker.Stop()

		s := NewSemaphore(cfg.RateLimit)
		const maxRetries = 3
		httpClient := resty.New().SetRetryCount(maxRetries).SetRetryAfter(retryAfterFunc()).SetRetryMaxWaitTime(1000 * time.Second)
		for range ticker.C {
			s.Acquire()
			go func() {
				defer s.Release()

				m.Lock()
				mCopy := copyMetrics(m)
				m.resetPollCount()
				m.Unlock()

				if err := sendReport(cfg.ServerAddr, cfg.Key, mCopy, httpClient); err != nil {
					logger.Error().Str("error", err.Error()).Msg("Error on sending metrics")

					// Корректирующее действие, если метрики в итоге не попали на сервер: значение дельты PollCount
					// возвращается в структуру metrics
					m.Lock()
					m.addPollCount(*mCopy["PollCount"].Delta)
					m.Unlock()
				}
			}()
		}
	}()
	wg.Wait()
}

func retryAfterFunc() func(client *resty.Client, r *resty.Response) (time.Duration, error) {
	const (
		firstRetryDelay   = 1 * time.Second
		otherRetriesDelay = 2 * time.Second
	)

	var retryCount int
	return func(client *resty.Client, r *resty.Response) (time.Duration, error) {
		if retryCount == 0 {
			retryCount++
			return firstRetryDelay, nil
		}
		return otherRetriesDelay, nil
	}
}

func copyMetrics(m *metrics) map[string]model.Metrics {
	mDataCopy := make(map[string]model.Metrics, len(m.data))
	for n, v := range m.data {
		mDataCopy[n] = v
	}
	return mDataCopy
}

func pollMetrics(m *metrics) {
	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)

	m.data["Alloc"] = convertUint64ToGaugeMetric("Alloc", memStats.Alloc)
	m.data["BuckHashSys"] = convertUint64ToGaugeMetric("BuckHashSys", memStats.BuckHashSys)
	m.data["Frees"] = convertUint64ToGaugeMetric("Frees", memStats.Frees)
	m.data["GCCPUFraction"] = convertFloat64ToGaugeMetric("GCCPUFraction", memStats.GCCPUFraction)
	m.data["GCSys"] = convertUint64ToGaugeMetric("GCSys", memStats.GCSys)
	m.data["HeapAlloc"] = convertUint64ToGaugeMetric("HeapAlloc", memStats.HeapAlloc)
	m.data["HeapIdle"] = convertUint64ToGaugeMetric("HeapIdle", memStats.HeapIdle)
	m.data["HeapInuse"] = convertUint64ToGaugeMetric("HeapInuse", memStats.HeapInuse)
	m.data["HeapObjects"] = convertUint64ToGaugeMetric("HeapObjects", memStats.HeapObjects)
	m.data["HeapReleased"] = convertUint64ToGaugeMetric("HeapReleased", memStats.HeapReleased)
	m.data["HeapSys"] = convertUint64ToGaugeMetric("HeapSys", memStats.HeapSys)
	m.data["LastGC"] = convertUint64ToGaugeMetric("LastGC", memStats.LastGC)
	m.data["Lookups"] = convertUint64ToGaugeMetric("Lookups", memStats.Lookups)
	m.data["MCacheInuse"] = convertUint64ToGaugeMetric("MCacheInuse", memStats.MCacheInuse)
	m.data["MCacheSys"] = convertUint64ToGaugeMetric("MCacheSys", memStats.MCacheSys)
	m.data["MSpanInuse"] = convertUint64ToGaugeMetric("MSpanInuse", memStats.MSpanInuse)
	m.data["MSpanSys"] = convertUint64ToGaugeMetric("MSpanSys", memStats.MSpanSys)
	m.data["Mallocs"] = convertUint64ToGaugeMetric("Mallocs", memStats.Mallocs)
	m.data["NextGC"] = convertUint64ToGaugeMetric("NextGC", memStats.NextGC)
	m.data["NumForcedGC"] = convertUint32ToGaugeMetric("NumForcedGC", memStats.NumForcedGC)
	m.data["NumGC"] = convertUint32ToGaugeMetric("NumGC", memStats.NumGC)
	m.data["OtherSys"] = convertUint64ToGaugeMetric("OtherSys", memStats.OtherSys)
	m.data["PauseTotalNs"] = convertUint64ToGaugeMetric("PauseTotalNs", memStats.PauseTotalNs)
	m.data["StackInuse"] = convertUint64ToGaugeMetric("StackInuse", memStats.StackInuse)
	m.data["StackSys"] = convertUint64ToGaugeMetric("StackSys", memStats.StackSys)
	m.data["Sys"] = convertUint64ToGaugeMetric("Sys", memStats.Sys)
	m.data["TotalAlloc"] = convertUint64ToGaugeMetric("TotalAlloc", memStats.TotalAlloc)
	m.data["RandomValue"] = convertUint32ToGaugeMetric("RandomValue", rand.Uint32())
}

func convertUint64ToGaugeMetric(name string, v uint64) model.Metrics {
	float64Value := float64(v)
	return model.Metrics{ID: name, MType: model.Gauge, Value: &float64Value}
}

func convertFloat64ToGaugeMetric(name string, v float64) model.Metrics {
	return model.Metrics{ID: name, MType: model.Gauge, Value: &v}
}

func convertUint32ToGaugeMetric(name string, v uint32) model.Metrics {
	float64Value := float64(v)
	return model.Metrics{ID: name, MType: model.Gauge, Value: &float64Value}
}

func pollUtilMetrics(m *metrics, logger zerolog.Logger) {
	if vm, err := mem.VirtualMemory(); err != nil {
		logger.Error().Str("error", err.Error()).Msg("Error on polling virtual memory")
	} else {
		m.data["TotalMemory"] = convertUint64ToGaugeMetric("TotalMemory", vm.Total)
		m.data["FreeMemory"] = convertUint64ToGaugeMetric("FreeMemory", vm.Free)
	}

	if pct, err := cpu.Percent(0, true); err != nil {
		logger.Error().Str("error", err.Error()).Msg("Error on polling CPU utilization")
	} else {
		for i, p := range pct {
			name := fmt.Sprintf("CPUutilization%d", i+1)
			m.data[name] = convertFloat64ToGaugeMetric(name, p)
		}
	}
}

func sendReport(serverAddr, key string, metrics map[string]model.Metrics, httpClient *resty.Client) error {
	metricsSlice := make([]model.Metrics, 0, len(metrics))
	for _, v := range metrics {
		metricsSlice = append(metricsSlice, v)
	}
	if err := sendMetrics(serverAddr, key, metricsSlice, httpClient); err != nil {
		return err
	}
	return nil
}

func sendMetrics(serverAddr, key string, metrics []model.Metrics, httpClient *resty.Client) error {
	jsonBz, err := marshal(metrics)
	if err != nil {
		return err
	}
	gzipBz, err := compress(jsonBz)
	if err != nil {
		return err
	}

	serverAddr = addDefaultURLSchema(serverAddr)
	URL, err := url.JoinPath(serverAddr, "/updates")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}
	r := httpClient.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetBody(gzipBz)
	if key != "" {
		hashBz := sha256.Sum256(jsonBz)
		r.SetHeader("HashSHA256", fmt.Sprintf("%x", hashBz))
	}
	_, err = r.Post(URL)
	if err != nil {
		return fmt.Errorf("failed to send the request: %w", err)
	}
	return nil
}

func marshal(metrics []model.Metrics) ([]byte, error) {
	jsonBz, err := json.Marshal(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics: %w", err)
	}
	return jsonBz, nil
}

func compress(data []byte) ([]byte, error) {
	var gzipBuf bytes.Buffer
	zw := gzip.NewWriter(&gzipBuf)
	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("failed to gzip data: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip data writer: %w", err)
	}
	return gzipBuf.Bytes(), nil
}

func addDefaultURLSchema(URL string) string {
	if strings.HasPrefix(URL, "https://") || strings.HasPrefix(URL, "http://") {
		return URL
	}
	hp := strings.Split(URL, ":")
	host := hp[0]
	if len(host) == 0 {
		host = "localhost"
	}
	port := hp[1]
	urlPrefix := ""
	if host == "localhost" {
		urlPrefix = "http://"
	} else {
		urlPrefix = "https://"
	}
	return urlPrefix + host + ":" + port
}
