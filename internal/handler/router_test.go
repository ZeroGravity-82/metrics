package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"zerogravity-82/metrics/internal/service"
)

func TestUpdateMetricHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		contentType    string
		action         string
		mType          string
		mName          string
		mValue         string
		wantResultCode int
	}{
		{
			name:           "fail when method is not allowed",
			method:         http.MethodPut,
			contentType:    "text/plain",
			action:         "update",
			mType:          "gauge",
			mName:          "RandomValue",
			mValue:         "12345",
			wantResultCode: http.StatusMethodNotAllowed,
		},
		{
			name:           "fail with unsupported media type",
			method:         http.MethodPost,
			contentType:    "application/json",
			action:         "update",
			mType:          "gauge",
			mName:          "RandomValue",
			mValue:         "12345",
			wantResultCode: http.StatusUnsupportedMediaType,
		},
		{
			name:           "fail when URL is malformed",
			method:         http.MethodPost,
			contentType:    "text/plain",
			action:         "update/update",
			mType:          "gauge",
			mName:          "RandomValue",
			mValue:         "12345",
			wantResultCode: http.StatusNotFound,
		},
		{
			name:           "fail when metric name is missing",
			method:         http.MethodPost,
			contentType:    "text/plain",
			action:         "update",
			mType:          "gauge",
			mName:          "",
			mValue:         "12345",
			wantResultCode: http.StatusNotFound,
		},
		{
			name:           "fail when action is not supported",
			method:         http.MethodPost,
			contentType:    "text/plain",
			action:         "unsupported",
			mType:          "gauge",
			mName:          "RandomValue",
			mValue:         "12345",
			wantResultCode: http.StatusNotFound,
		},
		{
			name:           "fail when counter delta value is invalid",
			method:         http.MethodPost,
			contentType:    "text/plain",
			action:         "update",
			mType:          "counter",
			mName:          "PollCount",
			mValue:         "5abc",
			wantResultCode: http.StatusBadRequest,
		},
		{
			name:           "fail when gauge value is invalid",
			method:         http.MethodPost,
			contentType:    "text/plain",
			action:         "update",
			mType:          "gauge",
			mName:          "GCCPUFraction",
			mValue:         "0.5abc",
			wantResultCode: http.StatusBadRequest,
		},
		{
			name:           "fail with unsupported metric type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			action:         "update",
			mType:          "unsupported",
			mName:          "GCCPUFraction",
			mValue:         "0.5",
			wantResultCode: http.StatusBadRequest,
		},
		{
			name:           "can update counter metric",
			method:         http.MethodPost,
			contentType:    "text/plain",
			action:         "update",
			mType:          "counter",
			mName:          "PollCount",
			mValue:         "15",
			wantResultCode: http.StatusOK,
		},
		{
			name:           "can update gauge metric",
			method:         http.MethodPost,
			contentType:    "text/plain",
			action:         "update",
			mType:          "gauge",
			mName:          "GCCPUFraction",
			mValue:         "0.5",
			wantResultCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			target := fmt.Sprintf("/%s/%s/%s/%s", tt.action, tt.mType, tt.mName, tt.mValue)
			r := httptest.NewRequest(tt.method, target, http.NoBody)
			r.Header.Set("Content-Type", tt.contentType)

			ms := service.NewMemStorage()
			handler := UpdateMetricHandler(ms)
			w := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(w, r)

			// Assert
			res := w.Result()
			assert.Equal(t, tt.wantResultCode, res.StatusCode)
		})
	}
}
