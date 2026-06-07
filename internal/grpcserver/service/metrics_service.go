package service

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"zerogravity-82/metrics/internal/model"
	pb "zerogravity-82/metrics/internal/proto"
	"zerogravity-82/metrics/internal/repository"
)

// Storage абстрагирует хранилище метрик.
type Storage interface {
	UpdateMetrics(ctx context.Context, metrics []model.Metrics) error
}

// AuditPublisher публикует события аудита об успешных обновлениях метрик.
type AuditPublisher interface {
	PublishLog(ctx context.Context, now time.Time, ip string, models ...model.Metrics)
}

type MetricsService struct {
	pb.UnimplementedMetricsServer
	s             Storage
	a             AuditPublisher
	trustedSubnet string
	logger        zerolog.Logger
}

func NewMetricsService(s Storage, a AuditPublisher, trustedSubnet string, logger zerolog.Logger) *MetricsService {
	return &MetricsService{
		s:             s,
		a:             a,
		trustedSubnet: trustedSubnet,
		logger:        logger,
	}
}

func (m *MetricsService) UpdateMetrics(
	ctx context.Context,
	in *pb.UpdateMetricsRequest,
) (*pb.UpdateMetricsResponse, error) {
	metrics := make([]model.Metrics, 0, len(in.GetMetrics()))
	for _, metric := range in.GetMetrics() {
		switch metric.GetType() {
		case pb.Metric_COUNTER:
			metrics = append(metrics, model.Metrics{
				ID:    metric.GetId(),
				MType: model.Counter,
				Delta: new(metric.GetDelta()),
				Value: nil,
			})
		case pb.Metric_GAUGE:
			metrics = append(metrics, model.Metrics{
				ID:    metric.GetId(),
				MType: model.Gauge,
				Delta: nil,
				Value: new(metric.GetValue()),
			})
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported metric type: %s", metric.GetType())
		}
	}

	err := m.s.UpdateMetrics(ctx, metrics)
	if err != nil {
		if errors.Is(err, repository.ErrMetricNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, repository.ErrUnsupportedMetricType) ||
			errors.Is(err, repository.ErrInvalidMetricType) ||
			errors.Is(err, repository.ErrInvalidMetricValue) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		logError(err, "Update metrics error", m.logger)
		return nil, status.Error(codes.Internal, "Internal Server Error")
	}

	var remoteAddr string
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		values := md.Get("x-real-ip")
		if len(values) > 0 {
			remoteAddr = values[0]
		}
	}
	m.a.PublishLog(ctx, time.Now(), remoteAddr, metrics...)
	return &pb.UpdateMetricsResponse{}, nil
}

func logError(err error, msg string, logger zerolog.Logger) {
	logger.Error().Err(err).Msg(msg)
}
