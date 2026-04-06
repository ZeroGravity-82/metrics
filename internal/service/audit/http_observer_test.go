package audit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/model"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Test_update_CanUpdateWithSingleAttempt проверяет успешную с первой попытки отправку.
func Test_update_CanUpdateWithSingleAttempt(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		u, err := url.Parse(r.URL.String())
		require.NoError(t, err)
		assert.Equal(t, "audit.com", u.Host)
		assert.Equal(t, http.MethodPost, r.Method)
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		assert.JSONEq(
			t,
			`{"ts":1775450609,"metrics":["CPUutilization9","FreeMemory"],"ip_address":"127.0.0.1"}`,
			string(body),
		)

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(nil)),
			Header:     make(http.Header),
		}, nil
	})

	o := NewHTTPObserver("https://audit.com")
	o.httpClient.client.HTTPClient.Transport = transport
	log := model.AuditLog{
		TS:      1775450609,
		Metrics: []string{"CPUutilization9", "FreeMemory"},
		IP:      "127.0.0.1",
	}

	// Act
	err := o.update(context.Background(), log)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int32(1), calls.Load())
}

// Test_update_CanUpdateWithRetriesOn5xxAndFinalSuccess проверяет успешную с третьей попытки отправку после 500-х
// ошибок.
func Test_update_CanUpdateWithRetriesOn5xxAndFinalSuccess(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		require.NoError(t, err)
		assert.JSONEq(t, `{"ts":1,"metrics":["m"],"ip_address":"127.0.0.1"}`, string(body))

		if n < 3 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString("internal server error")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(nil)),
			Header:     make(http.Header),
		}, nil
	})

	o := NewHTTPObserver("https://audit.com")
	// Ускоряем тесты: нам важны попытки, не ожидания.
	o.httpClient.client.RetryWaitMin = 1 * time.Millisecond
	o.httpClient.client.RetryWaitMax = 1 * time.Millisecond
	o.httpClient.client.HTTPClient.Transport = transport

	// Act
	err := o.update(context.Background(), model.AuditLog{TS: 1, Metrics: []string{"m"}, IP: "127.0.0.1"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int32(3), calls.Load())
}

// Test_update_CanUpdateWithRetriesOnTransportErrorAndFinalSuccess проверяет успешную с третьей попытки отправку после
// транспортной ошибки.
func Test_update_CanUpdateWithRetriesOnTransportErrorAndFinalSuccess(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	boom := errors.New("boom")
	transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if n < 3 {
			return nil, boom
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(nil)),
			Header:     make(http.Header),
		}, nil
	})

	o := NewHTTPObserver("https://audit.com")
	// Ускоряем тесты: нам важны попытки, не ожидания.
	o.httpClient.client.RetryWaitMin = 1 * time.Millisecond
	o.httpClient.client.RetryWaitMax = 1 * time.Millisecond
	o.httpClient.client.HTTPClient.Transport = transport

	// Act
	err := o.update(context.Background(), model.AuditLog{TS: 1, Metrics: []string{"m"}, IP: "127.0.0.1"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int32(3), calls.Load())
}

// Test_update_FailOn4xxWithoutRetries проверяет отсутствие повторных попыток отправки при 400-й ошибке.
func Test_update_FailOn4xxWithoutRetries(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewBufferString("bad request")),
			Header:     make(http.Header),
		}, nil
	})

	o := NewHTTPObserver("https://audit.com")
	o.httpClient.client.HTTPClient.Transport = transport

	// Act
	err := o.update(context.Background(), model.AuditLog{TS: 1, Metrics: []string{"m"}, IP: "127.0.0.1"})

	// Assert
	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load())
}

// Test_update_FailWhenRetriesExhausted проверяет, что после исчерпания попыток отправки возвращается ошибка.
func Test_update_FailWhenRetriesExhausted(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewBufferString("internal server error")),
			Header:     make(http.Header),
		}, nil
	})

	o := NewHTTPObserver("https://audit.com")
	// Ускоряем тесты: нам важны попытки, не ожидания.
	o.httpClient.client.RetryWaitMin = 1 * time.Millisecond
	o.httpClient.client.RetryWaitMax = 1 * time.Millisecond
	o.httpClient.client.HTTPClient.Transport = transport

	// Act
	err := o.update(context.Background(), model.AuditLog{TS: 1, Metrics: []string{"m"}, IP: "127.0.0.1"})

	// Assert
	require.Error(t, err)
	assert.Equal(t, int32(maxRetries+1), calls.Load())
}
