package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/encryption"
	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/service/audit"
)

var (
	sinkInt    int
	sinkErr    error
	sinkMetric model.Metrics
)

type discardResponseWriter struct {
	header      http.Header
	statusCode  int
	wroteHeader bool
}

func newDiscardResponseWriter() *discardResponseWriter {
	return &discardResponseWriter{header: make(http.Header)}
}

func (w *discardResponseWriter) Header() http.Header {
	return w.header
}

func (w *discardResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return io.Discard.Write(p)
}

func (w *discardResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.statusCode = statusCode
	w.wroteHeader = true
}

type nopStorage struct{}

func (nopStorage) UpdateMetric(context.Context, model.Metrics) error    { return nil }
func (nopStorage) UpdateMetrics(context.Context, []model.Metrics) error { return nil }
func (nopStorage) GetMetric(context.Context, string, string) (model.Metrics, error) {
	return model.Metrics{}, nil
}
func (nopStorage) GetAll(context.Context) (map[string]model.Metrics, error) {
	return map[string]model.Metrics{}, nil
}
func (nopStorage) Ping(context.Context) error { return nil }
func (nopStorage) Close() error               { return nil }

type nopAuditPublisher struct{}

func (nopAuditPublisher) Register(observers ...audit.Observer)                            {}
func (nopAuditPublisher) Deregister(o audit.Observer) error                               { return nil }
func (nopAuditPublisher) PublishLog(context.Context, time.Time, string, ...model.Metrics) {}

func BenchmarkSigningResponseWriter_CalculateSignature(b *testing.B) {
	// Setup
	inner := newDiscardResponseWriter()
	key := "secret"
	logger := zerolog.Nop()
	data := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)

	// Measure
	for b.Loop() {
		b.StopTimer()
		writer := newSigningResponseWriter(inner, key, logger)
		sinkInt, sinkErr = writer.Write(data)
		b.StartTimer()

		writer.WriteHeader(http.StatusOK)
	}
}

func BenchmarkMetricRouter_validateSignature(b *testing.B) {
	// Setup
	body := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	signatureKey := "secret"
	sig := hex.EncodeToString(generateSignature(body, signatureKey))

	// Measure
	for b.Loop() {
		sinkErr = validateSignature(sig, body, signatureKey)
	}
}

func BenchmarkMetricRouter_buildMetric(b *testing.B) {
	b.Run("gauge metric", func(b *testing.B) {
		// Measure
		for b.Loop() {
			sinkMetric, sinkErr = buildMetric(model.Gauge, "RandomValue", "123.45")
		}
	})
	b.Run("counter metric", func(b *testing.B) {
		// Measure
		for b.Loop() {
			sinkMetric, sinkErr = buildMetric(model.Counter, "PollCount", "777")
		}
	})
}

func BenchmarkMetricRouter_UpdateRoute(b *testing.B) {
	// Setup
	logger := zerolog.Nop()
	r := MetricRouter(nopStorage{}, nopAuditPublisher{}, "", "", logger)
	payload := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)

	// Measure
	for b.Loop() {
		b.StopTimer()
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := newDiscardResponseWriter()
		b.StartTimer()

		r.ServeHTTP(w, req)

		sinkInt = w.statusCode
	}
}

func BenchmarkMetricRouter_UpdateRoute_GzipIn_GzipOut(b *testing.B) {
	// Setup
	logger := zerolog.Nop()
	r := MetricRouter(nopStorage{}, nopAuditPublisher{}, "", "", logger)
	payload := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	gzPayload := gzipBody(payload)

	// Measure
	for b.Loop() {
		b.StopTimer()
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(gzPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")
		w := newDiscardResponseWriter()
		b.StartTimer()

		r.ServeHTTP(w, req)
		sinkInt = w.statusCode
	}
}

func gzipBody(payload []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	sinkInt, sinkErr = zw.Write(payload)
	sinkErr = zw.Close()
	return buf.Bytes()
}

func BenchmarkMetricRouter_UpdateRoute_WithSignature(b *testing.B) {
	// Setup
	logger := zerolog.Nop()
	signatureKey := "secret"
	cryptoKeyPath := ""
	r := MetricRouter(nopStorage{}, nopAuditPublisher{}, signatureKey, cryptoKeyPath, logger)
	payload := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	sig := hex.EncodeToString(generateSignature(payload, signatureKey))

	// Measure
	for b.Loop() {
		b.StopTimer()
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HashSHA256", sig)
		w := newDiscardResponseWriter()
		b.StartTimer()

		r.ServeHTTP(w, req)
		sinkInt = w.statusCode
	}
}

func BenchmarkMetricRouter_UpdateRoute_WithEncryption(b *testing.B) {
	// Setup
	logger := zerolog.Nop()
	signatureKey := ""
	privateKeyPath, publicKeyPath := writeBenchmarkRSAKeyPair(b)
	r := MetricRouter(nopStorage{}, nopAuditPublisher{}, signatureKey, privateKeyPath, logger)
	payload := []byte(`{"id":"PollCounter","type":"counter","delta":145}`)
	encryptedData, encryptedKey, err := encryption.Encrypt(payload, publicKeyPath)
	if err != nil {
		b.Fatalf("failed to encrypt payload: %v", err)
	}

	// Measure
	for b.Loop() {
		b.StopTimer()
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(encryptedData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(encryption.XEncryptedHeaderName, "aes-gcm+rsa-oaep-sha256")
		req.Header.Set(encryption.XEncryptedKeyHeaderName, base64.StdEncoding.EncodeToString(encryptedKey))
		w := newDiscardResponseWriter()
		b.StartTimer()

		r.ServeHTTP(w, req)
		sinkInt = w.statusCode
	}
}

func writeBenchmarkRSAKeyPair(b *testing.B) (privateKeyPath, publicKeyPath string) {
	b.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatalf("failed to generate private key: %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		b.Fatalf("failed to marshal public key: %v", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	dir := b.TempDir()
	privateKeyPath = filepath.Join(dir, "private.pem")
	publicKeyPath = filepath.Join(dir, "public.pem")

	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0o600); err != nil {
		b.Fatalf("failed to write private key: %v", err)
	}
	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0o600); err != nil {
		b.Fatalf("failed to write public key: %v", err)
	}

	return privateKeyPath, publicKeyPath
}
