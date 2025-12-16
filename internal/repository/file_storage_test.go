package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/model"
)

func TestNewFileStorage_FailInstantiateWhenCantOpenCfgFile(t *testing.T) {
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

func TestNewFileStorage_CanInstantiateWithoutRestore(t *testing.T) {
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

func TestNewFileStorage_CanInstantiateWithRestore(t *testing.T) {
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
			name: "can restore with valid JSON string",
			fileContent: `[
				{"id":"PollCount","type":"counter","delta":777},
				{"id":"RandomValue","type":"gauge","value":123.45}
			]`,
			wantError: false,
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
		m := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(777), Value: nil}

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
		m := model.Metrics{ID: "RandomValue", MType: model.Gauge, Delta: nil, Value: float64Pointer(123.45)}

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
		isEqual, err := JSONEqualFile(t, `[{"id":"RandomValue","type":"gauge","value":123.45}]`, tempFile.Name())
		require.NoError(t, err)
		assert.True(t, isEqual)
	})
}

func JSONEqualFile(t *testing.T, JSON string, fileName string) (bool, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return false, err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	var metricsFromFile []model.Metrics
	if err = dec.Decode(&metricsFromFile); err != nil {
		return false, err
	}

	r := strings.NewReader(JSON)
	dec = json.NewDecoder(r)
	var metricsFromJson []model.Metrics
	if err = dec.Decode(&metricsFromJson); err != nil {
		return false, err
	}

	if len(metricsFromFile) != len(metricsFromJson) {
		return false, nil
	}

	for _, m := range metricsFromFile {
		if !assert.Contains(t, metricsFromJson, m) {
			return false, nil
		}
	}
	return true, nil
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
		[]byte(`[
			{"id":"PollCount","type":"counter","delta":777},
			{"id":"RandomValue","type":"gauge","value":123.45}]
		`),
		0666,
	)
	require.NoError(t, err)
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)
	m := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(111), Value: nil}

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
	isEqual, err := JSONEqualFile(
		t,
		`[
			{"id":"PollCount","type":"counter","delta":888},
			{"id":"RandomValue","type":"gauge","value":123.45}
		]`,
		tempFile.Name(),
	)
	require.NoError(t, err)
	assert.True(t, isEqual)
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
		[]byte(`[
			{"id":"PollCount","type":"counter","delta":777},
			{"id":"RandomValue","type":"gauge","value":123.45}
		]`),
		0666,
	)
	require.NoError(t, err)
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)
	m := model.Metrics{ID: "RandomValue", MType: model.Gauge, Delta: nil, Value: float64Pointer(234.56)}

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
	isEqual, err := JSONEqualFile(
		t,
		`[
			{"id":"PollCount","type":"counter","delta":777},
			{"id":"RandomValue","type":"gauge","value":234.56}
		]`,
		tempFile.Name(),
	)
	require.NoError(t, err)
	assert.True(t, isEqual)
}

func TestUpdateMetricsInFileStorage_FailEvenWithOneSingleInvalidMetric(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	err = os.WriteFile(
		tempFile.Name(),
		[]byte(`[
			{"id":"PollCount","type":"counter","delta":777},
			{"id":"RandomValue","type":"gauge","value":123.45}
		]`),
		0666,
	)
	require.NoError(t, err)
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)
	mu := []model.Metrics{
		{ID: "FooCounter", MType: "unsupported", Delta: int64Pointer(123), Value: nil},
		{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(111), Value: nil},
		{ID: "RandomValue", MType: model.Gauge, Delta: nil, Value: float64Pointer(234.56)},
	}

	// Act
	err = fs.UpdateMetrics(ctx, mu)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedMetricType)
	assert.Equal(t, fs.metrics["PollCount"], model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(777),
		Value: nil,
	})
	assert.Equal(t, fs.metrics["RandomValue"], model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(123.45),
	})
	assert.Len(t, fs.metrics, 2)
	isEqual, err := JSONEqualFile(
		t,
		`[
			{"id":"PollCount","type":"counter","delta":777},
			{"id":"RandomValue","type":"gauge","value":123.45}
		]`,
		tempFile.Name(),
	)
	require.NoError(t, err)
	assert.True(t, isEqual)
}

func TestUpdateMetricsInFileStorage_CanAddNewAndUpdateExistingMetrics(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	err = os.WriteFile(
		tempFile.Name(),
		[]byte(`[
			{"id":"PollCount","type":"counter","delta":777},
			{"id":"RandomValue","type":"gauge","value":123.45}
		]`),
		0666,
	)
	require.NoError(t, err)
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)
	mu := []model.Metrics{
		{ID: "FooCounter", MType: model.Counter, Delta: int64Pointer(123), Value: nil},
		{ID: "BarGauge", MType: model.Gauge, Delta: nil, Value: float64Pointer(0.5)},
		{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(111), Value: nil},
		{ID: "RandomValue", MType: model.Gauge, Delta: nil, Value: float64Pointer(234.56)},
		{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(222), Value: nil},
		{ID: "RandomValue", MType: model.Gauge, Delta: nil, Value: float64Pointer(345.67)},
	}

	// Act
	err = fs.UpdateMetrics(ctx, mu)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fs.metrics["FooCounter"], model.Metrics{
		ID:    "FooCounter",
		MType: model.Counter,
		Delta: int64Pointer(123),
		Value: nil,
	})
	assert.Equal(t, fs.metrics["BarGauge"], model.Metrics{
		ID:    "BarGauge",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(0.5),
	})
	assert.Equal(t, fs.metrics["PollCount"], model.Metrics{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: int64Pointer(1110),
		Value: nil,
	})
	assert.Equal(t, fs.metrics["RandomValue"], model.Metrics{
		ID:    "RandomValue",
		MType: model.Gauge,
		Delta: nil,
		Value: float64Pointer(345.67),
	})
	assert.Len(t, fs.metrics, 4)
	isEqual, err := JSONEqualFile(
		t,
		`[
			{"id":"PollCount","type":"counter","delta":1110},
			{"id":"RandomValue","type":"gauge","value":345.67},
			{"id":"FooCounter","type":"counter","delta":123},
			{"id":"BarGauge","type":"gauge","value":0.5}
		]`,
		tempFile.Name(),
	)
	require.NoError(t, err)
	assert.True(t, isEqual)
}

func TestGetMetricInFileStorage(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	err = os.WriteFile(
		tempFile.Name(),
		[]byte(`[
			{"id":"PollCount","type":"counter","delta":777},
			{"id":"RandomValue","type":"gauge","value":123.45}
		]`),
		0666,
	)
	require.NoError(t, err)
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)

	t.Run("fail when not found by name", func(t *testing.T) {
		// Act
		_, err := fs.GetMetric(ctx, model.Counter, "unknown")

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrMetricNotFound)
	})
	t.Run("fail when not found with same type", func(t *testing.T) {
		// Act
		_, err := fs.GetMetric(ctx, model.Gauge, "PollCount")

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrMetricNotFound)
	})
	t.Run("can get counter by name", func(t *testing.T) {
		// Act
		m, err := fs.GetMetric(ctx, model.Counter, "PollCount")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "PollCount", m.ID)
		assert.Equal(t, "counter", m.MType)
		assert.Equal(t, int64(777), *m.Delta)
		assert.Nil(t, m.Value)
	})
	t.Run("can get gauge by name", func(t *testing.T) {
		// Act
		m, err := fs.GetMetric(ctx, model.Gauge, "RandomValue")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "RandomValue", m.ID)
		assert.Equal(t, "gauge", m.MType)
		assert.Nil(t, m.Delta)
		assert.Equal(t, float64(123.45), *m.Value)
	})
}

func TestGetAllInFileStorage(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	err = os.WriteFile(
		tempFile.Name(),
		[]byte(`[
			{"id":"PollCount","type":"counter","delta":777},
			{"id":"RandomValue","type":"gauge","value":123.45}
		]`),
		0666,
	)
	require.NoError(t, err)
	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
		Restore:         true,
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)

	// Act
	metrics, err := fs.GetAll(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, metrics, fs.metrics)
}

func TestPingInFileStorage(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)

	// Act
	err = fs.Ping(ctx)

	// Assert
	require.NoError(t, err)
}

func TestCloseInFileStorage(t *testing.T) {
	// Arrange
	tempFile, err := os.CreateTemp("", "metrics*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	cfg := config.ServerConfig{
		FileStoragePath: tempFile.Name(),
	}
	fs, err := NewFileStorage(cfg)
	require.NoError(t, err)

	// Act
	err = fs.Close()

	// Assert
	require.NoError(t, err)
}
