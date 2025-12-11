package repository

import (
	"fmt"

	"zerogravity-82/metrics/internal/model"
)

func validateMetric(m model.Metrics) error {
	if len(m.ID) == 0 {
		return fmt.Errorf("%w: empty name", ErrMetricNotFound)
	}
	if m.MType == model.Counter && (m.Delta == nil || m.Value != nil) ||
		m.MType == model.Gauge && (m.Value == nil || m.Delta != nil) {
		return fmt.Errorf("%w", ErrInvalidMetricValue)
	}
	if m.MType != model.Counter && m.MType != model.Gauge {
		return fmt.Errorf("%w: %s", ErrUnsupportedMetricType, m.MType)
	}
	return nil
}
