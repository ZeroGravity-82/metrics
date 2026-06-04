package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
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
	"zerogravity-82/metrics/internal/encryption"
	"zerogravity-82/metrics/internal/model"
)

const requestTimeout = 10 * time.Second

type metrics struct {
	mu   sync.Mutex
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

func (m *metrics) pollMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	// #nosec G404 -- RandomValue is not about security
	m.data["RandomValue"] = convertUint32ToGaugeMetric("RandomValue", rand.Uint32())

	incrementPollCount(m)
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

func incrementPollCount(m *metrics) {
	v := *m.data["PollCount"].Delta
	v++
	m.data["PollCount"] = model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &v}
}

func (m *metrics) pollUtilMetrics(logger zerolog.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vm, err := mem.VirtualMemory(); err != nil {
		logger.Error().Err(err).Msg("Error on polling virtual memory")

	} else {
		m.data["TotalMemory"] = convertUint64ToGaugeMetric("TotalMemory", vm.Total)
		m.data["FreeMemory"] = convertUint64ToGaugeMetric("FreeMemory", vm.Free)
	}
	if pct, err := cpu.Percent(0, true); err != nil {
		logger.Error().Err(err).Msg("Error on polling CPU utilization")
	} else {
		for i, p := range pct {
			name := fmt.Sprintf("CPUutilization%d", i+1)
			m.data[name] = convertFloat64ToGaugeMetric(name, p)
		}
	}
}

func (m *metrics) copyMetricsAndResetPollCount() map[string]model.Metrics {
	mCopy := m.copyMetrics()
	m.resetPollCount()
	return mCopy
}

func (m *metrics) copyMetrics() map[string]model.Metrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	mCopy := make(map[string]model.Metrics, len(m.data))
	for n, v := range m.data {
		mCopy[n] = v
	}
	return mCopy
}

func (m *metrics) restorePollCount(delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	v := *m.data["PollCount"].Delta
	v += delta
	m.data["PollCount"] = model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &v}
}

// Run запускает цикл опроса метрик и периодически отправляет их на сервер.
//
// Функция блокируется, пока процесс не будет остановлен.
func Run(ctx context.Context, cfg config.AgentConfig, logger zerolog.Logger) {
	m := newMetrics()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Duration(cfg.PollInterval))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.pollMetrics()
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Duration(cfg.PollInterval))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.pollUtilMetrics(logger)
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Duration(cfg.ReportInterval))
		defer ticker.Stop()

		s := NewSemaphore(cfg.RateLimit)
		const maxRetries = 3
		httpClient := resty.New().SetRetryCount(maxRetries).SetRetryAfter(retryAfterFunc())

		for {
			select {
			case <-ctx.Done():
				mCopy := m.copyMetrics()
				if *mCopy["PollCount"].Delta > 0 {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
					defer cancel()

					if err := sendReport(
						shutdownCtx,
						cfg.ServerAddr,
						cfg.SignatureKey,
						cfg.CryptoKeyPath,
						mCopy,
						httpClient,
					); err != nil && !errors.Is(err, context.Canceled) {
						logger.Error().Err(err).Msg("Error on sending metrics")
					}
				}
				return
			case <-ticker.C:
				select {
				case <-ctx.Done(): // Защита на случай одновременной отмены контекста и срабатывания тикера.
					continue
				default:
				}

				s.Acquire()
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer s.Release()

					requestCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
					defer cancel()

					mCopy := m.copyMetricsAndResetPollCount()
					if err := sendReport(
						requestCtx,
						cfg.ServerAddr,
						cfg.SignatureKey,
						cfg.CryptoKeyPath,
						mCopy,
						httpClient,
					); err != nil {
						logger.Error().Err(err).Msg("Error on sending metrics")

						// Корректирующее действие, если метрики в итоге не попали на сервер: значение дельты PollCount
						// возвращается в структуру metrics
						m.restorePollCount(*mCopy["PollCount"].Delta)
					}
				}()
			}
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
	return func(_ *resty.Client, r *resty.Response) (time.Duration, error) {
		if retryCount == 0 {
			retryCount++
			return firstRetryDelay, nil
		}
		return otherRetriesDelay, nil
	}
}

func sendReport(
	ctx context.Context,
	serverAddr,
	signatureKey,
	cryptoKeyPath string,
	metrics map[string]model.Metrics,
	httpClient *resty.Client,
) error {
	metricsSlice := make([]model.Metrics, 0, len(metrics))
	for _, v := range metrics {
		metricsSlice = append(metricsSlice, v)
	}
	if err := sendMetrics(ctx, serverAddr, signatureKey, cryptoKeyPath, metricsSlice, httpClient); err != nil {
		return err
	}
	return nil
}

func sendMetrics(
	ctx context.Context,
	serverAddr,
	signatureKey,
	cryptoKeyPath string,
	metrics []model.Metrics,
	httpClient *resty.Client,
) error {
	const xEncryptedHeaderValue = "aes-gcm+rsa-oaep-sha256"

	jsonBz, err := marshal(metrics)
	if err != nil {
		return err
	}
	gzipBz, err := compress(jsonBz)
	if err != nil {
		return err
	}
	var encryptedGzipBz, encryptedKeyBz, bodyBz []byte
	if cryptoKeyPath != "" {
		encryptedGzipBz, encryptedKeyBz, err = encryption.Encrypt(gzipBz, cryptoKeyPath)
		if err != nil {
			return fmt.Errorf("failed to encrypt metrics: %w", err)
		}
		bodyBz = encryptedGzipBz
	} else {
		bodyBz = gzipBz
	}

	serverAddr = addDefaultURLSchema(serverAddr)
	urlPath, err := url.JoinPath(serverAddr, "/updates")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}
	r := httpClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetBody(bodyBz)
	if signatureKey != "" {
		r.SetHeader("HashSHA256", generateHexEncodedSignature(jsonBz, signatureKey))
	}
	if cryptoKeyPath != "" {
		r.SetHeader(encryption.XEncryptedHeaderName, xEncryptedHeaderValue)
		r.SetHeader(encryption.XEncryptedKeyHeaderName, base64.StdEncoding.EncodeToString(encryptedKeyBz))
	}
	ip, err := outboundIPFor(serverAddr)
	if err != nil {
		return fmt.Errorf("failed to determine local IP address: %w", err)
	}
	r.SetHeader("X-Real-IP", ip)
	_, err = r.Post(urlPath)
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

func addDefaultURLSchema(urlPath string) string {
	if strings.HasPrefix(urlPath, "https://") || strings.HasPrefix(urlPath, "http://") {
		return urlPath
	}
	hp := strings.Split(urlPath, ":")
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

func generateHexEncodedSignature(data []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	signature := hex.EncodeToString(h.Sum(nil))
	return signature
}

func outboundIPFor(target string) (string, error) {
	u, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}
	conn, err := net.Dial("udp", u.Hostname()+":"+u.Port())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("unexpected local address type %T", conn.LocalAddr())
	}

	return localAddr.IP.String(), nil
}
