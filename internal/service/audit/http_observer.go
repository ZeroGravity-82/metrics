package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"zerogravity-82/metrics/internal/model"
)

const (
	// maxRetries задает количество повторных попыток отправки запроса.
	maxRetries int = 3

	// requestTimeout задает тайм-аут на каждую попытку отправки запроса.
	requestTimeout time.Duration = 30 * time.Second

	// httpClientTimeout является подстраховкой от зависаний транспорта/чтения тела ответа.
	httpClientTimeout time.Duration = 60 * time.Second
)

// HTTPObserver является HTTP-клиентом и отправляет сообщение события аудита POST-запросом в удаленный приемник.
//
// Повторные запросы и экспоненциальная задержка (backoff) реализованы во внутреннем транспортном клиенте.
type HTTPObserver struct {
	httpClient *retryingHTTPClient
	url        string
}

// NewHTTPObserver создает HTTPObserver с указанным URL удаленного приемника.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		httpClient: newRetryingHTTPClient(
			httpClientTimeout,
			maxRetries,
			200*time.Millisecond,
			2*time.Second,
			requestTimeout,
		),
		url: url,
	}
}

func (o *HTTPObserver) update(ctx context.Context, log model.AuditLog) error {
	b, err := json.Marshal(log)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(resp.Status)
	}
	return nil
}
