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
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/model"
)

type metrics struct {
	memStat     map[string]float64
	pollCount   int64
	randomValue float64
}

func Run(cfg config.AgentConfig, logger zerolog.Logger) {
	const maxRetries = 3

	m := metrics{}
	m.memStat = make(map[string]float64)

	httpClient := resty.New().SetRetryCount(maxRetries).SetRetryAfter(retryAfterFunc())
	lastSentTime := time.Now()
	for {
		pollMetrics(&m)
		m.pollCount++
		time.Sleep(time.Duration(cfg.PollInterval) * time.Second)

		if time.Since(lastSentTime) >= time.Duration(cfg.ReportInterval)*time.Second {
			lastSentTime = time.Now()
			sendReport(cfg.ServerAddr, cfg.Key, &m, httpClient, logger)
			m.pollCount = 0
		}
	}
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

func pollMetrics(m *metrics) {
	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)

	m.memStat["Alloc"] = float64(memStats.Alloc)
	m.memStat["BuckHashSys"] = float64(memStats.BuckHashSys)
	m.memStat["Frees"] = float64(memStats.Frees)
	m.memStat["GCCPUFraction"] = memStats.GCCPUFraction
	m.memStat["GCSys"] = float64(memStats.GCSys)
	m.memStat["HeapAlloc"] = float64(memStats.HeapAlloc)
	m.memStat["HeapIdle"] = float64(memStats.HeapIdle)
	m.memStat["HeapInuse"] = float64(memStats.HeapInuse)
	m.memStat["HeapObjects"] = float64(memStats.HeapObjects)
	m.memStat["HeapReleased"] = float64(memStats.HeapReleased)
	m.memStat["HeapSys"] = float64(memStats.HeapSys)
	m.memStat["LastGC"] = float64(memStats.LastGC)
	m.memStat["Lookups"] = float64(memStats.Lookups)
	m.memStat["MCacheInuse"] = float64(memStats.MCacheInuse)
	m.memStat["MCacheSys"] = float64(memStats.MCacheSys)
	m.memStat["MSpanInuse"] = float64(memStats.MSpanInuse)
	m.memStat["MSpanSys"] = float64(memStats.MSpanSys)
	m.memStat["Mallocs"] = float64(memStats.Mallocs)
	m.memStat["NextGC"] = float64(memStats.NextGC)
	m.memStat["NumForcedGC"] = float64(memStats.NumForcedGC)
	m.memStat["NumGC"] = float64(memStats.NumGC)
	m.memStat["OtherSys"] = float64(memStats.OtherSys)
	m.memStat["PauseTotalNs"] = float64(memStats.PauseTotalNs)
	m.memStat["StackInuse"] = float64(memStats.StackInuse)
	m.memStat["StackSys"] = float64(memStats.StackSys)
	m.memStat["Sys"] = float64(memStats.Sys)
	m.memStat["TotalAlloc"] = float64(memStats.TotalAlloc)
	m.randomValue = float64(rand.Uint32())
}

func sendReport(serverAddr, key string, metrics *metrics, httpClient *resty.Client, logger zerolog.Logger) {
	metricsSlice := make([]model.Metrics, 0)

	for name, value := range metrics.memStat {
		m := model.Metrics{ID: name, MType: model.Gauge, Value: &value}
		metricsSlice = append(metricsSlice, m)
	}
	mPollCount := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &metrics.pollCount}
	mRandomValue := model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &metrics.randomValue}
	metricsSlice = append(metricsSlice, mPollCount, mRandomValue)

	if err := sendMetrics(serverAddr, key, metricsSlice, httpClient); err != nil {
		logger.Error().Str("error", err.Error()).Msg("Error on sending metrics")
	}
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
		return nil, fmt.Errorf("failed to gzip metrics: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip metrics writer: %w", err)
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
