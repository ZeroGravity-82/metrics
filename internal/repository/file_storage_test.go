package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileStorage_FailWhenCantOpenCfgFile(t *testing.T) {
	// Arrange
	cfg := config.ServerConfig{
		FileStoragePath: "",
		Restore:         false,
	}

	// Act
	_, err := NewFileStorage(cfg)

	// Assert
	require.Error(t, err)
	var pathError *os.PathError
	ok := errors.As(err, &pathError)
	require.True(t, ok)

}

func TestNewFileStorage_WithoutRestore(t *testing.T) {
	// Arrange
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	err = os.WriteFile(tempFile.Name(), []byte(`[{"id":"PollCount","type":"counter","delta":777}]`), 0666)
	require.NoError(t, err)

	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         false,
	}

	// Act
	fs, err := NewFileStorage(cfg)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, tempFile.Name(), fs.file.Name())
	assert.Empty(t, fs.metrics)
}

func TestNewFileStorage_WithRestore(t *testing.T) {
	// Arrange
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	tests := []struct {
		name        string
		fileContent string
		wantError   bool
	}{
		{
			name:        "fail to restore with invalid JSON string",
			fileContent: "_",
			wantError:   true,
		},
		{
			name:        "can restore with empty file",
			fileContent: "",
			wantError:   false,
		},
		{
			name:        "can restore with valid JSON string",
			fileContent: `[{"id":"PollCount","type":"counter","delta":777},{"id":"RandomValue","type":"gauge","value":123.45}]`,
			wantError:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			err = os.WriteFile(cfg.FileStoragePath, []byte(tt.fileContent), 0666)
			require.NoError(t, err)

			// Act
			fs, err := NewFileStorage(cfg)

			// Assert
			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tempFile.Name(), fs.file.Name())

			if len(tt.fileContent) == 0 {
				assert.Empty(t, fs.metrics)
				return
			}

			assert.Len(t, fs.metrics, 2)
			assert.Equal(t, model.Metrics{
				ID:    "PollCount",
				MType: "counter",
				Delta: int64Pointer(777),
				Value: nil,
			}, fs.metrics["PollCount"])
			assert.Equal(t, model.Metrics{
				ID:    "RandomValue",
				MType: "gauge",
				Delta: nil,
				Value: float64Pointer(123.45),
			}, fs.metrics["RandomValue"])
		})
	}
}

func TestUpdateMetricInFileStorage_CanAddNewMetric(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         false,
	}
	t.Run("can add new counter", func(t *testing.T) {
		// Arrange
		fs, err := NewFileStorage(cfg)
		require.NoError(t, err)
		m := model.Metrics{
			ID:    "PollCount",
			MType: model.Counter,
			Delta: int64Pointer(777),
			Value: nil,
		}

		// Act
		err = fs.UpdateMetric(ctx, m)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, fs.metrics, "PollCount")
		assert.Equal(
			t,
			model.Metrics{ID: "PollCount", MType: "counter", Delta: int64Pointer(777), Value: nil},
			fs.metrics["PollCount"],
		)
		assert.Len(t, fs.metrics, 1)
		fileContentBz, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		assert.JSONEq(t, `[{"id":"PollCount","type":"counter","delta":777}]`, string(fileContentBz))
	})
	t.Run("can add new gauge", func(t *testing.T) {
		// Arrange
		fs, err := NewFileStorage(cfg)
		require.NoError(t, err)
		m := model.Metrics{
			ID:    "RandomValue",
			MType: model.Gauge,
			Delta: nil,
			Value: float64Pointer(123.45),
		}

		// Act
		err = fs.UpdateMetric(ctx, m)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, fs.metrics, "RandomValue")
		assert.Equal(
			t,
			model.Metrics{ID: "RandomValue", MType: "gauge", Delta: nil, Value: float64Pointer(123.45)},
			fs.metrics["RandomValue"],
		)
		assert.Len(t, fs.metrics, 1)
		fileContentBz, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		assert.JSONEq(t, `[{"id":"RandomValue","type":"gauge","value":123.45}]`, string(fileContentBz))
	})
}

func TestUpdateMetricInFileStorage_CanUpdateExistingCounter(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	err = os.WriteFile(
		tempFile.Name(),
		[]byte(`[{"id":"PollCount","type":"counter","delta":777},{"id":"RandomValue","type":"gauge","value":123.45}]`),
		0666,
	)
	require.NoError(t, err)
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)
	m := model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(111),
		Value: nil,
	}

	// Act
	err = fs.UpdateMetric(ctx, m)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, fs.metrics, "PollCount")
	assert.Equal(
		t,
		model.Metrics{ID: "PollCount", MType: "counter", Delta: int64Pointer(888), Value: nil},
		fs.metrics["PollCount"],
	)
	assert.Equal(
		t,
		model.Metrics{ID: "RandomValue", MType: "gauge", Delta: nil, Value: float64Pointer(123.45)},
		fs.metrics["RandomValue"],
	)
	assert.Len(t, fs.metrics, 2)
	fileContentBz, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`[{"id":"PollCount","type":"counter","delta":888},{"id":"RandomValue","type":"gauge","value":123.45}]`,
		string(fileContentBz),
	)
}

func TestUpdateMetricInFileStorage_CanUpdateExistingGauge(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	err = os.WriteFile(
		tempFile.Name(),
		[]byte(`[{"id":"PollCount","type":"counter","delta":777},{"id":"RandomValue","type":"gauge","value":123.45}]`),
		0666,
	)
	require.NoError(t, err)
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)
	m := model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(234.56),
	}

	// Act
	err = fs.UpdateMetric(ctx, m)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, fs.metrics, "PollCount")
	assert.Equal(
		t,
		model.Metrics{ID: "PollCount", MType: "counter", Delta: int64Pointer(777), Value: nil},
		fs.metrics["PollCount"],
	)
	assert.Equal(
		t,
		model.Metrics{ID: "RandomValue", MType: "gauge", Delta: nil, Value: float64Pointer(234.56)},
		fs.metrics["RandomValue"],
	)
	assert.Len(t, fs.metrics, 2)
	fileContentBz, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`[{"id":"PollCount","type":"counter","delta":777},{"id":"RandomValue","type":"gauge","value":234.56}]`,
		string(fileContentBz),
	)
}
