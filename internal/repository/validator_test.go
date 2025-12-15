package repository

import (
	"testing"

	"zerogravity-82/metrics/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestValidateMetric(t *testing.T) {
	// Arrange
	tests := []struct {
		name      string
		m         model.Metrics
		wantError error
	}{
		{
			name: "fail with empty name",
			m: model.Metrics{
				ID:    "",
				MType: model.Counter,
				Delta: int64Pointer(777),
				Value: nil,
			},
			wantError: ErrMetricNotFound,
		},
		{
			name: "fail with unsupported metric type",
			m: model.Metrics{
				ID:    "PollCount",
				MType: "unsupported",
				Delta: int64Pointer(777),
				Value: nil,
			},
			wantError: ErrUnsupportedMetricType,
		},
		{
			name: "fail with counter without delta",
			m: model.Metrics{
				ID:    "PollCount",
				MType: model.Counter,
				Delta: nil,
				Value: nil,
			},
			wantError: ErrInvalidMetricValue,
		},
		{
			name: "fail with counter with redundant value",
			m: model.Metrics{
				ID:    "PollCount",
				MType: model.Counter,
				Delta: int64Pointer(777),
				Value: float64Pointer(234.56),
			},
			wantError: ErrInvalidMetricValue,
		},
		{
			name: "fail with gauge without value",
			m: model.Metrics{
				ID:    "RandomValue",
				MType: model.Gauge,
				Delta: nil,
				Value: nil,
			},
			wantError: ErrInvalidMetricValue,
		},
		{
			name: "fail with gauge with redundant delta",
			m: model.Metrics{
				ID:    "RandomValue",
				MType: model.Gauge,
				Delta: int64Pointer(777),
				Value: float64Pointer(234.56),
			},
			wantError: ErrInvalidMetricValue,
		},
		{
			name: "can process valid counter",
			m: model.Metrics{
				ID:    "PollCounter",
				MType: model.Counter,
				Delta: int64Pointer(777),
				Value: nil,
			},
			wantError: nil,
		},
		{
			name: "can process valid gauge",
			m: model.Metrics{
				ID:    "RandomValue",
				MType: model.Gauge,
				Delta: nil,
				Value: float64Pointer(234.56),
			},
			wantError: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := validateMetric(tt.m)

			// Assert
			assert.ErrorIs(t, err, tt.wantError)
		})
	}
}
