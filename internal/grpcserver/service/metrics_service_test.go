package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"zerogravity-82/metrics/internal/model"
	pb "zerogravity-82/metrics/internal/proto"
	"zerogravity-82/metrics/internal/repository"
)

type storageStub struct {
	err     error
	called  bool
	metrics []model.Metrics
}

func (s *storageStub) UpdateMetrics(_ context.Context, metrics []model.Metrics) error {
	s.called = true
	s.metrics = append([]model.Metrics(nil), metrics...)
	return s.err
}

type auditPublisherStub struct {
	called  bool
	ip      string
	metrics []model.Metrics
}

func (a *auditPublisherStub) PublishLog(_ context.Context, _ time.Time, ip string, metrics ...model.Metrics) {
	a.called = true
	a.ip = ip
	a.metrics = append([]model.Metrics(nil), metrics...)
}

// TestMetricsService_UpdateMetrics_CanUpdateBatchAndPublishAudit проверяет, что сервис сохраняет батч метрик и
// публикует аудит с IP-адресом из метаданных gRPC.
func TestMetricsService_UpdateMetrics_CanUpdateBatchAndPublishAudit(t *testing.T) {
	// Arrange
	storage := &storageStub{}
	auditPublisher := &auditPublisherStub{}
	service := NewMetricsService(storage, auditPublisher, "", zerolog.Nop())
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-real-ip", "192.168.1.10"))
	req := pb.UpdateMetricsRequest_builder{Metrics: []*pb.Metric{
		pb.Metric_builder{Id: "PollCount", Type: pb.Metric_COUNTER, Delta: 5}.Build(),
		pb.Metric_builder{Id: "RandomValue", Type: pb.Metric_GAUGE, Value: 42.5}.Build(),
	}}.Build()

	// Act
	resp, err := service.UpdateMetrics(ctx, req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, storage.called)
	require.Len(t, storage.metrics, 2)
	assert.Equal(t, "PollCount", storage.metrics[0].ID)
	assert.Equal(t, model.Counter, storage.metrics[0].MType)
	require.NotNil(t, storage.metrics[0].Delta)
	assert.Equal(t, int64(5), *storage.metrics[0].Delta)
	assert.Nil(t, storage.metrics[0].Value)
	assert.Equal(t, "RandomValue", storage.metrics[1].ID)
	assert.Equal(t, model.Gauge, storage.metrics[1].MType)
	assert.Nil(t, storage.metrics[1].Delta)
	require.NotNil(t, storage.metrics[1].Value)
	assert.Equal(t, 42.5, *storage.metrics[1].Value)

	require.True(t, auditPublisher.called)
	assert.Equal(t, "192.168.1.10", auditPublisher.ip)
	assert.Equal(t, storage.metrics, auditPublisher.metrics)
}

// TestMetricsService_UpdateMetrics_ReturnsInvalidArgumentForUnsupportedMetricType проверяет, что сервис возвращает
// InvalidArgument для неизвестного protobuf-типа метрики.
func TestMetricsService_UpdateMetrics_ReturnsInvalidArgumentForUnsupportedMetricType(t *testing.T) {
	// Arrange
	storage := &storageStub{}
	auditPublisher := &auditPublisherStub{}
	service := NewMetricsService(storage, auditPublisher, "", zerolog.Nop())
	req := pb.UpdateMetricsRequest_builder{Metrics: []*pb.Metric{
		pb.Metric_builder{Id: "BrokenMetric", Type: pb.Metric_MType(99)}.Build(),
	}}.Build()

	// Act
	resp, err := service.UpdateMetrics(context.Background(), req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.False(t, storage.called)
	assert.False(t, auditPublisher.called)
}

// TestMetricsService_UpdateMetrics_MapsStorageErrorsToGRPCCodes проверяет, что ошибки хранилища преобразуются в
// ожидаемые gRPC-коды.
func TestMetricsService_UpdateMetrics_MapsStorageErrorsToGRPCCodes(t *testing.T) {
	// Arrange
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{
			name:     "not found",
			err:      repository.ErrMetricNotFound,
			wantCode: codes.NotFound,
		},
		{
			name:     "unsupported metric type",
			err:      repository.ErrUnsupportedMetricType,
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid metric type",
			err:      repository.ErrInvalidMetricType,
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid metric value",
			err:      repository.ErrInvalidMetricValue,
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "unexpected storage error",
			err:      errors.New("database is unavailable"),
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			storage := &storageStub{err: tt.err}
			auditPublisher := &auditPublisherStub{}
			service := NewMetricsService(storage, auditPublisher, "", zerolog.Nop())
			req := pb.UpdateMetricsRequest_builder{Metrics: []*pb.Metric{
				pb.Metric_builder{Id: "PollCount", Type: pb.Metric_COUNTER, Delta: 1}.Build(),
			}}.Build()

			// Act
			resp, err := service.UpdateMetrics(context.Background(), req)

			// Assert
			require.Error(t, err)
			assert.Nil(t, resp)
			assert.Equal(t, tt.wantCode, status.Code(err))
			assert.True(t, storage.called)
			assert.False(t, auditPublisher.called)
		})
	}
}
