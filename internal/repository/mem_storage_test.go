package repository

import (
	"context"
	"testing"

	"zerogravity-82/metrics/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func int64Pointer(v int64) *int64 {
	return &v
}

func float64Pointer(v float64) *float64 {
	return &v
}

func TestNewMemStorage(t *testing.T) {
	// Act
	ms := NewMemStorage()

	// Assert
	assert.NotNil(t, ms)
	assert.Empty(t, ms.metrics)
}

func TestUpdateMetric_FailWithEmptyName(t *testing.T) {
	// Arrange
	ctx := context.Background()
	ms := NewMemStorage()
	m := model.Metrics{
		ID:    "",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	}

	// Act
	err := ms.UpdateMetric(ctx, m)

	// Assert
	assert.ErrorIs(t, err, ErrMetricNotFound)
	assert.Len(t, ms.metrics, 0)
}

func TestUpdateMetric_FailUpdateWithSameNameButAnotherType(t *testing.T) {
	// Arrange
	ctx := context.Background()
	ms := NewMemStorage()
	err := ms.UpdateMetric(ctx, model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	})
	require.NoError(t, err)
	err = ms.UpdateMetric(ctx, model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(123.45),
	})
	require.NoError(t, err)

	tests := []struct {
		name string
		m    model.Metrics
	}{
		{
			name: "fail update counter with same name gauge",
			m: model.Metrics{
				ID:    "PollCount",
				MType: model.Gauge,
				Delta: nil,
				Value: float64Pointer(123.45),
			},
		},
		{
			name: "fail update gauge with same name counter",
			m: model.Metrics{
				ID:    "RandomValue",
				MType: model.Counter,
				Delta: int64Pointer(777),
				Value: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err = ms.UpdateMetric(ctx, tt.m)

			// Assert
			assert.ErrorIs(t, err, ErrInvalidMetricType)
			assert.Len(t, ms.metrics, 2)
		})
	}
}

func TestUpdateMetric_CanAddNewMetric(t *testing.T) {
	// Arrange
	ctx := context.Background()
	t.Run("can add new counter", func(t *testing.T) {
		// Arrange
		ms := NewMemStorage()
		m := model.Metrics{
			ID:    "PollCount",
			MType: model.Counter,
			Delta: int64Pointer(777),
			Value: nil,
		}

		// Act
		err := ms.UpdateMetric(ctx, m)

		// Assert
		assert.NoError(t, err)
		assert.Contains(t, ms.metrics, "PollCount")
		assert.Equal(
			t,
			model.Metrics{ID: "PollCount", MType: "counter", Delta: int64Pointer(777), Value: nil},
			ms.metrics["PollCount"],
		)
		assert.Len(t, ms.metrics, 1)
	})
	t.Run("can add new gauge", func(t *testing.T) {
		// Arrange
		ms := NewMemStorage()
		m := model.Metrics{
			ID:    "RandomValue",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(123.45),
		}

		// Act
		err := ms.UpdateMetric(ctx, m)

		// Assert
		assert.NoError(t, err)
		assert.Contains(t, ms.metrics, "RandomValue")
		assert.Equal(
			t,
			model.Metrics{ID: "RandomValue", MType: "gauge", Delta: nil, Value: float64Pointer(123.45)},
			ms.metrics["RandomValue"],
		)
		assert.Len(t, ms.metrics, 1)
	})
}

func TestUpdateMetric_CanUpdateExistingMetric(t *testing.T) {
	// Arrange
	ctx := context.Background()
	t.Run("can update existing counter", func(t *testing.T) {
		// Arrange
		ms := NewMemStorage()
		m := model.Metrics{
			ID:    "PollCount",
			MType: model.Counter,
			Delta: int64Pointer(777),
			Value: nil,
		}
		err := ms.UpdateMetric(ctx, m)
		require.NoError(t, err)
		mu := model.Metrics{
			ID:    "PollCount",
			MType: model.Counter,
			Delta: int64Pointer(111),
			Value: nil,
		}

		// Act
		err = ms.UpdateMetric(ctx, mu)

		// Assert
		assert.NoError(t, err)
		assert.Contains(t, ms.metrics, "PollCount")
		assert.Equal(
			t,
			model.Metrics{ID: "PollCount", MType: "counter", Delta: int64Pointer(888), Value: nil},
			ms.metrics["PollCount"],
		)
		assert.Len(t, ms.metrics, 1)
	})
	t.Run("can update existing gauge", func(t *testing.T) {
		// Arrange
		ms := NewMemStorage()
		m := model.Metrics{
			ID:    "RandomValue",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(123.45),
		}
		err := ms.UpdateMetric(ctx, m)
		mu := model.Metrics{
			ID:    "RandomValue",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(234.56),
		}

		// Act
		err = ms.UpdateMetric(ctx, mu)

		// Assert
		assert.NoError(t, err)
		assert.Contains(t, ms.metrics, "RandomValue")
		assert.Equal(
			t,
			model.Metrics{ID: "RandomValue", MType: "gauge", Delta: nil, Value: float64Pointer(234.56)},
			ms.metrics["RandomValue"],
		)
		assert.Len(t, ms.metrics, 1)
	})
}

func TestUpdateMetrics_FailEvenWithOneSingleInvalidMetric(t *testing.T) {
	// Arrange
	ctx := context.Background()
	ms := NewMemStorage()
	err := ms.UpdateMetric(ctx, model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	})
	require.NoError(t, err)
	err = ms.UpdateMetric(ctx, model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(123.45),
	})
	require.NoError(t, err)
	mu := []model.Metrics{
		{
			ID:    "FooCounter",
			MType: "unsupported",
			Delta: int64Pointer(123),
			Value: nil,
		},
		{
			ID:    "PollCount",
			MType: model.Counter,
			Delta: int64Pointer(111),
			Value: nil,
		},
		{
			ID:    "RandomValue",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(234.56),
		},
	}

	// Act
	err = ms.UpdateMetrics(ctx, mu)

	// Assert
	assert.ErrorIs(t, err, ErrUnsupportedMetricType)
	assert.Equal(t, ms.metrics["PollCount"], model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	})
	assert.Equal(t, ms.metrics["RandomValue"], model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(123.45),
	})
	assert.Len(t, ms.metrics, 2)
}

func TestUpdateMetrics_FailUpdateWithSameNameButAnotherType(t *testing.T) {
	// Arrange
	ctx := context.Background()
	ms := NewMemStorage()
	err := ms.UpdateMetric(ctx, model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	})
	require.NoError(t, err)
	err = ms.UpdateMetric(ctx, model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(123.45),
	})
	require.NoError(t, err)
	mu := []model.Metrics{
		{
			ID:    "PollCount",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(0.5),
		},
		{
			ID:    "RandomValue",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(234.56),
		},
	}

	// Act
	err = ms.UpdateMetrics(ctx, mu)

	// Assert
	assert.ErrorIs(t, err, ErrInvalidMetricType)
	assert.Equal(t, ms.metrics["PollCount"], model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	})
	assert.Equal(t, ms.metrics["RandomValue"], model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(123.45),
	})
	assert.Len(t, ms.metrics, 2)
}

func TestUpdateMetrics_CanAddNewAndUpdateExistingMetrics(t *testing.T) {
	// Arrange
	ctx := context.Background()
	ms := NewMemStorage()
	err := ms.UpdateMetric(ctx, model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	})
	require.NoError(t, err)
	err = ms.UpdateMetric(ctx, model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(123.45),
	})
	require.NoError(t, err)
	mu := []model.Metrics{
		{
			ID:    "FooCounter",
			MType: model.Counter,
			Delta: int64Pointer(123),
			Value: nil,
		},
		{
			ID:    "BarGauge",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(0.5),
		},
		{
			ID:    "PollCount",
			MType: model.Counter,
			Delta: int64Pointer(111),
			Value: nil,
		},
		{
			ID:    "RandomValue",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(234.56),
		},
	}

	// Act
	err = ms.UpdateMetrics(ctx, mu)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, ms.metrics["FooCounter"], model.Metrics{
		ID:    "FooCounter",
		MType: model.Counter,
		Delta: int64Pointer(123),
		Value: nil,
	})
	assert.Equal(t, ms.metrics["BarGauge"], model.Metrics{
		ID:    "BarGauge",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(0.5),
	})
	assert.Equal(t, ms.metrics["PollCount"], model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(888),
		Value: nil,
	})
	assert.Equal(t, ms.metrics["RandomValue"], model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(234.56),
	})
	assert.Len(t, ms.metrics, 4)
}

func TestGetAll(t *testing.T) {
	// Arrange
	ctx := context.Background()
	ms := NewMemStorage()
	err := ms.UpdateMetric(ctx, model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	})
	require.NoError(t, err)
	err = ms.UpdateMetric(ctx, model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(123.45),
	})
	require.NoError(t, err)

	// Act
	metrics, err := ms.GetAll(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, metrics, ms.metrics)
}

func TestPing(t *testing.T) {
	// Arrange
	ctx := context.Background()
	ms := NewMemStorage()

	// Act
	err := ms.Ping(ctx)

	// Assert
	assert.NoError(t, err)
}

func TestClose(t *testing.T) {
	// Arrange
	ms := NewMemStorage()

	// Act
	err := ms.Close()

	// Assert
	assert.NoError(t, err)
}
