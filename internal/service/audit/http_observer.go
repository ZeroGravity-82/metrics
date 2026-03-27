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

// HTTPObserver отправляет сообщение события аудита POST-запросом в удаленный приемник.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver создает HTTPObserver с указанным URL удаленного приемника.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{url: url, client: &http.Client{Timeout: 3 * time.Second}}
}

func (s *HTTPObserver) update(ctx context.Context, log model.AuditLog) error {
	b, err := json.Marshal(log)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(resp.Status)
	}
	return nil
}
