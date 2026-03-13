package audit

import (
	"context"
	"net"
	"time"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/model"
)

const defaultQueueSize = 1024

// Observer получает событие аудита и обрабатывает его по своему усмотрению.
type Observer interface {
	update(ctx context.Context, log model.AuditLog) error
}

// AsyncPublisher публикует сообщения событий аудита асинхронно.
//
// При переполнении внутренней очереди события отбрасываются.
type AsyncPublisher struct {
	observers []Observer
	queue     chan model.AuditLog
	logger    zerolog.Logger
}

func NewAsyncPublisher(logger zerolog.Logger, observers ...Observer) *AsyncPublisher {
	return &AsyncPublisher{
		observers: observers,
		queue:     make(chan model.AuditLog, defaultQueueSize),
		logger:    logger,
	}
}

func (a *AsyncPublisher) PublishLog(_ context.Context, now time.Time, ip string, metrics ...model.Metrics) {
	if len(a.observers) == 0 {
		return
	}

	ip = normalizeIP(ip)
	log := convertMetricsToAuditLog(now, ip, metrics)

	select {
	case a.queue <- log:
	default:
		// Не блокируем вызывающий код: при переполнении очереди логи отбрасываются.
	}
}

func normalizeIP(v string) string {
	host, _, err := net.SplitHostPort(v)
	if err == nil {
		return host
	}
	return v
}

func convertMetricsToAuditLog(now time.Time, ip string, metrics []model.Metrics) model.AuditLog {
	log := model.AuditLog{
		TS:      now.Unix(),
		Metrics: make([]string, 0, len(metrics)),
		IP:      ip,
	}
	for _, m := range metrics {
		log.Metrics = append(log.Metrics, m.ID)
	}
	return log
}

// Run запускает фоновый воркер и завершает его при отмене контекста.
func (a *AsyncPublisher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case log := <-a.queue:
			a.notify(ctx, log)
		}
	}
}

func (a *AsyncPublisher) notify(ctx context.Context, log model.AuditLog) {
	for _, o := range a.observers {
		func() {
			defer func() {
				// Паника одного наблюдателя не должно влиять на остальных наблюдателей.
				if r := recover(); r != nil {
					a.logger.Error().Interface("panic", r).Msg("audit observer panicked")
				}
			}()
			if err := o.update(ctx, log); err != nil {
				a.logger.Error().Err(err).Msg("audit observer update failed")
			}
		}()
	}
}
