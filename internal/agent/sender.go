package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/encryption"
	"zerogravity-82/metrics/internal/model"
	pb "zerogravity-82/metrics/internal/proto"
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
	return model.Metrics{ID: name, MType: model.Gauge, Value: new(float64(v))}
}

func convertFloat64ToGaugeMetric(name string, v float64) model.Metrics {
	return model.Metrics{ID: name, MType: model.Gauge, Value: &v}
}

func convertUint32ToGaugeMetric(name string, v uint32) model.Metrics {
	return model.Metrics{ID: name, MType: model.Gauge, Value: new(float64(v))}
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
func Run(ctx context.Context, cfg config.AgentConfig, logger zerolog.Logger) error {
	m := newMetrics()

	eg, groupCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		ticker := time.NewTicker(time.Duration(cfg.PollInterval))
		defer ticker.Stop()

		for {
			select {
			case <-groupCtx.Done():
				return nil
			case <-ticker.C:
				m.pollMetrics()
			}
		}
	})
	eg.Go(func() error {
		ticker := time.NewTicker(time.Duration(cfg.PollInterval))
		defer ticker.Stop()

		for {
			select {
			case <-groupCtx.Done():
				return nil
			case <-ticker.C:
				m.pollUtilMetrics(logger)
			}
		}
	})
	eg.Go(func() error {
		ticker := time.NewTicker(time.Duration(cfg.ReportInterval))
		defer ticker.Stop()

		s := NewSemaphore(cfg.RateLimit)

		send, cleanup, err := buildSendFunc(cfg, logger)
		if err != nil {
			return fmt.Errorf("failed to build send function: %w", err)
		}
		defer cleanup()

		for {
			select {
			case <-groupCtx.Done():
				mCopy := m.copyMetrics()
				if *mCopy["PollCount"].Delta > 0 {
					if err := send(mCopy); err != nil {
						logger.Error().Err(err).Msg("Error on sending metrics")
					}
				}
				return nil
			case <-ticker.C:
				select {
				case <-groupCtx.Done(): // Защита на случай одновременной отмены контекста и срабатывания тикера.
					continue
				default:
				}

				s.Acquire()
				eg.Go(func() error {
					defer s.Release()

					mCopy := m.copyMetricsAndResetPollCount()
					if err := send(mCopy); err != nil {
						logger.Error().Err(err).Msg("Error on sending metrics")

						// Корректирующее действие, если метрики в итоге не попали на сервер: значение дельты PollCount
						// возвращается в структуру metrics
						m.restorePollCount(*mCopy["PollCount"].Delta)
					}
					return nil
				})
			}
		}
	})
	return eg.Wait()
}

type sendFunc func(metrics map[string]model.Metrics) error

func buildSendFunc(cfg config.AgentConfig, logger zerolog.Logger) (sendFunc, func(), error) {
	const maxRetries = 3

	caCert, err := os.ReadFile(cfg.CACertPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read ca certificate: %w", err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCert); !ok {
		return nil, nil, fmt.Errorf("failed to append ca certificate: %w", err)
	}

	httpClient := resty.New().
		SetRetryCount(maxRetries).
		SetRetryAfter(retryAfterFunc()).
		SetTLSClientConfig(&tls.Config{RootCAs: caPool})

	send := func(metrics map[string]model.Metrics) error {
		return sendReportHTTP(cfg.HTTPServerAddr, cfg.SignatureKey, cfg.CryptoKeyPath, metrics, httpClient)
	}
	if cfg.GRPCServerAddr == "" {
		return send, func() {}, nil
	}

	grpcClientCredentials := credentials.NewTLS(&tls.Config{RootCAs: caPool})
	conn, err := grpc.NewClient(cfg.GRPCServerAddr, grpc.WithTransportCredentials(grpcClientCredentials))
	if err != nil {
		return send, func() {}, fmt.Errorf("failed to create grpc client: %w", err)
	}

	grpcClient := pb.NewMetricsClient(conn)
	send = func(metrics map[string]model.Metrics) error {
		return sendReportGRPC(cfg.GRPCServerAddr, metrics, grpcClient)
	}
	return send, func() {
		if err = conn.Close(); err != nil {
			logger.Error().Err(err).Msg("Error on closing gRPC client connection")
		}
	}, nil
}

func sendReportGRPC(
	serverAddr string,
	metrics map[string]model.Metrics,
	client pb.MetricsClient,
) error {
	metricsSlice := make([]model.Metrics, 0, len(metrics))
	for _, v := range metrics {
		metricsSlice = append(metricsSlice, v)
	}
	return sendMetricsGRPC(serverAddr, metricsSlice, client)
}

func sendMetricsGRPC(serverAddr string, metrics []model.Metrics, client pb.MetricsClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	protoMetrics := make([]*pb.Metric, 0, len(metrics))
	for _, m := range metrics {
		protoMetric, err := buildProtoMetric(m)
		if err != nil {
			return err
		}
		protoMetrics = append(protoMetrics, protoMetric)
	}
	ip, err := outboundIPFor(serverAddr)
	if err != nil {
		return fmt.Errorf("failed to determine local IP address: %w", err)
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-real-ip", ip)
	req := pb.UpdateMetricsRequest_builder{Metrics: protoMetrics}.Build()
	if _, err = client.UpdateMetrics(ctx, req); err != nil {
		return fmt.Errorf("failed to send metrics via gRPC: %w", err)
	}
	return nil
}

func buildProtoMetric(m model.Metrics) (*pb.Metric, error) {
	switch m.MType {
	case model.Counter:
		var delta int64
		if m.Delta != nil {
			delta = *m.Delta
		}
		return pb.Metric_builder{Id: m.ID, Type: pb.Metric_COUNTER, Delta: delta}.Build(), nil
	case model.Gauge:
		var value float64
		if m.Value != nil {
			value = *m.Value
		}
		return pb.Metric_builder{Id: m.ID, Type: pb.Metric_GAUGE, Value: value}.Build(), nil
	default:
		return nil, fmt.Errorf("unsupported metric type: %s", m.MType)
	}
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

func sendReportHTTP(
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
	if err := sendMetricsHTTP(serverAddr, signatureKey, cryptoKeyPath, metricsSlice, httpClient); err != nil {
		return err
	}
	return nil
}

func sendMetricsHTTP(
	serverAddr,
	signatureKey,
	cryptoKeyPath string,
	metrics []model.Metrics,
	httpClient *resty.Client,
) error {
	const xEncryptedHeaderValue = "aes-gcm+rsa-oaep-sha256"

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

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
		return fmt.Errorf("failed to send metrics via HTTP: %w", err)
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
	return "https://" + host + ":" + port
}

func generateHexEncodedSignature(data []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	signature := hex.EncodeToString(h.Sum(nil))
	return signature
}

// outboundIPFor определяет локальный IP-адрес, через который агент будет обращаться к целевому серверу.
//
// Параметр target может быть HTTP URL вида "http://localhost:8080" или gRPC-адресом без схемы вида "localhost:3201".
func outboundIPFor(target string) (string, error) {
	targetAddr := target
	// Для HTTP URL адрес лежит в u.Host, а gRPC передает обычный host:port без схемы.
	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err != nil {
			return "", fmt.Errorf("failed to parse URL: %w", err)
		}
		targetAddr = u.Host
	}
	// Проверяем, что после нормализации остался адрес в формате host:port, пригодный для net.Dial.
	if _, _, err := net.SplitHostPort(targetAddr); err != nil {
		return "", fmt.Errorf("failed to parse target host and port: %w", err)
	}
	conn, err := net.Dial("udp", targetAddr)
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
