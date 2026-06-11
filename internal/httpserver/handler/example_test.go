package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/encryption"
	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/repository"
	"zerogravity-82/metrics/internal/service/audit"
)

func gzipJSON(v any) (*bytes.Buffer, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

func writeExampleRSAKeyPair() (privateKeyPath, publicKeyPath string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	dir, err := os.MkdirTemp("", "metrics-example-keys-*")
	if err != nil {
		return "", "", err
	}

	privateKeyPath = filepath.Join(dir, "private.pem")
	publicKeyPath = filepath.Join(dir, "public.pem")

	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0o600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0o600); err != nil {
		return "", "", err
	}
	return privateKeyPath, publicKeyPath, nil
}

// Example_updateValueText показывает работу с ручкой `POST /update/{type}/{name}/{value}`.
func Example_updateValueText() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	signatureKey := ""
	cryptoKeyPath := ""
	trustedSubnet := ""

	router, err := NewMetricRouter(store, auditPublisher, signatureKey, cryptoKeyPath, trustedSubnet, logger)
	if err != nil {
		fmt.Println("router error")
		return
	}
	ts := httptest.NewServer(router)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/update/gauge/RandomValue/123.45", http.NoBody)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := ts.Client().Do(req)
	if err != nil {
		fmt.Println("request error")
		return
	}
	_ = resp.Body.Close()
	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}

// Example_getValueText показывает работу с ручкой `GET /value/{type}/{name}`.
func Example_getValueText() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	_ = store.UpdateMetric(
		context.Background(),
		model.Metrics{ID: "PollCount", MType: model.Counter, Delta: func() *int64 { v := int64(777); return &v }()},
	)
	signatureKey := ""
	cryptoKeyPath := ""
	trustedSubnet := ""

	router, err := NewMetricRouter(store, auditPublisher, signatureKey, cryptoKeyPath, trustedSubnet, logger)
	if err != nil {
		fmt.Println("router error")
		return
	}
	ts := httptest.NewServer(router)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/value/counter/PollCount", http.NoBody)
	resp, _ := ts.Client().Do(req)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	fmt.Printf("%d %s\n", resp.StatusCode, string(body))

	// Output:
	// 200 777
}

// Example_updateValueJSON показывает работу с ручкой `POST /update`
func Example_updateValueJSON() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	signatureKey := ""
	cryptoKeyPath := ""
	trustedSubnet := ""

	router, err := NewMetricRouter(store, auditPublisher, signatureKey, cryptoKeyPath, trustedSubnet, logger)
	if err != nil {
		fmt.Println("router error")
		return
	}
	ts := httptest.NewServer(router)
	defer ts.Close()

	payload := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: func() *int64 { v := int64(5); return &v }()}
	buf, _ := gzipJSON(payload)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/update", buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, _ := ts.Client().Do(req)
	_ = resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}

// Example_updatesValuesJSON показывает работу с ручкой `POST /updates`
func Example_updatesValuesJSON() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	signatureKey := ""
	cryptoKeyPath := ""
	trustedSubnet := ""

	router, err := NewMetricRouter(store, auditPublisher, signatureKey, cryptoKeyPath, trustedSubnet, logger)
	if err != nil {
		fmt.Println("router error")
		return
	}
	ts := httptest.NewServer(router)
	defer ts.Close()

	payload := []model.Metrics{
		{ID: "GCCPUFraction", MType: model.Gauge, Value: func() *float64 { v := 0.5; return &v }()},
		{ID: "MyCounter", MType: model.Counter, Delta: func() *int64 { v := int64(10); return &v }()},
	}
	buf, _ := gzipJSON(payload)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/updates", buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, _ := ts.Client().Do(req)
	_ = resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}

// Example_updatesValuesJSON_signed показывает работу с ручкой `POST /updates` при наличии подписи
// запроса.
func Example_updatesValuesJSON_signed() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	signatureKey := "secret"
	cryptoKeyPath := ""
	trustedSubnet := ""

	router, err := NewMetricRouter(store, auditPublisher, signatureKey, cryptoKeyPath, trustedSubnet, logger)
	if err != nil {
		fmt.Println("router error")
		return
	}
	ts := httptest.NewServer(router)
	defer ts.Close()

	payload := []model.Metrics{
		{ID: "GCCPUFraction", MType: model.Gauge, Value: func() *float64 { v := 0.5; return &v }()},
		{ID: "MyCounter", MType: model.Counter, Delta: func() *int64 { v := int64(10); return &v }()},
	}
	jsonBody, _ := json.Marshal(payload)
	buf, _ := gzipJSON(payload)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/updates", buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	h := hmac.New(sha256.New, []byte(signatureKey))
	_, _ = h.Write(jsonBody)
	req.Header.Set("HashSHA256", fmt.Sprintf("%x", h.Sum(nil)))

	resp, _ := ts.Client().Do(req)
	_ = resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}

// Example_updatesValuesJSON_encrypted показывает работу с ручкой `POST /updates` при наличии шифрования
// запроса.
func Example_updatesValuesJSON_encrypted() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	signatureKey := ""
	privateKeyPath, publicKeyPath, _ := writeExampleRSAKeyPair()
	trustedSubnet := ""
	defer os.Remove(privateKeyPath)
	defer os.Remove(publicKeyPath)

	router, err := NewMetricRouter(store, auditPublisher, signatureKey, privateKeyPath, trustedSubnet, logger)
	if err != nil {
		fmt.Println("router error")
		return
	}
	ts := httptest.NewServer(router)
	defer ts.Close()

	payload := []model.Metrics{
		{ID: "GCCPUFraction", MType: model.Gauge, Value: func() *float64 { v := 0.5; return &v }()},
		{ID: "MyCounter", MType: model.Counter, Delta: func() *int64 { v := int64(10); return &v }()},
	}
	buf, _ := gzipJSON(payload)
	encryptedData, encryptedKey, _ := encryption.Encrypt(buf.Bytes(), publicKeyPath)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/updates", bytes.NewReader(encryptedData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set(encryption.XEncryptedHeaderName, "aes-gcm+rsa-oaep-sha256")
	req.Header.Set(encryption.XEncryptedKeyHeaderName, base64.StdEncoding.EncodeToString(encryptedKey))

	resp, _ := ts.Client().Do(req)
	_ = resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}

// Example_getValueJSON показывает работу с ручкой `POST /value`
func Example_getValueJSON() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	_ = store.UpdateMetric(
		context.Background(),
		model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: func() *float64 { v := 123.45; return &v }()},
	)
	signatureKey := ""
	cryptoKeyPath := ""
	trustedSubnet := ""

	router, err := NewMetricRouter(store, auditPublisher, signatureKey, cryptoKeyPath, trustedSubnet, logger)
	if err != nil {
		fmt.Println("router error")
		return
	}
	ts := httptest.NewServer(router)
	defer ts.Close()

	payload := model.Metrics{ID: "RandomValue", MType: model.Gauge}
	buf, _ := gzipJSON(payload)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/value", buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, _ := ts.Client().Do(req)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	fmt.Printf("%d %s\n", resp.StatusCode, string(bytes.TrimSpace(body)))

	// Output:
	// 200 {"id":"RandomValue","type":"gauge","value":123.45}
}

// Example_ping показывает работу с ручкой `GET /ping`
func Example_ping() {
	logger := zerolog.Nop()
	store := repository.NewMemStorage()
	auditPublisher := audit.NewAsyncPublisher(logger)
	signatureKey := ""
	cryptoKeyPath := ""
	trustedSubnet := ""

	router, err := NewMetricRouter(store, auditPublisher, signatureKey, cryptoKeyPath, trustedSubnet, logger)
	if err != nil {
		fmt.Println("router error")
		return
	}
	ts := httptest.NewServer(router)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/ping", http.NoBody)
	resp, _ := ts.Client().Do(req)
	_ = resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}
