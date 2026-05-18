package audit

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/model"
)

const (
	defaultQueueSize    = 1024
	maxConcurrentNotify = 10
)

// Observer абстрагирует наблюдателя за событиями аудита, который получает их и обрабатывает по своему усмотрению.
type Observer interface {
	update(ctx context.Context, log model.AuditLog) error
}

// AsyncPublisher публикует сообщения событий аудита асинхронно.
//
// При переполнении внутренней очереди события отбрасываются.
type AsyncPublisher struct {
	mu         sync.Mutex
	observers  []Observer
	queue      chan model.AuditLog
	logger     zerolog.Logger
	semaCh     chan struct{}
	notifyWG   sync.WaitGroup
	closing    bool
	shutdownCh chan struct{}
}

// NewAsyncPublisher создает AsyncPublisher с заданными наблюдателями.
func NewAsyncPublisher(logger zerolog.Logger, observers ...Observer) *AsyncPublisher {
	p := &AsyncPublisher{
		queue:      make(chan model.AuditLog, defaultQueueSize),
		logger:     logger,
		semaCh:     make(chan struct{}, maxConcurrentNotify),
		shutdownCh: make(chan struct{}),
	}
	p.Register(observers...)
	return p
}

// Register добавляет наблюдателей динамически.
//
// Повторная регистрация того же экземпляра (по ссылочному равенству) игнорируется.
func (a *AsyncPublisher) Register(observers ...Observer) {
	if len(observers) == 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, o := range observers {
		if o == nil {
			continue
		}
		// Не добавляем дубликаты.
		exists := false
		for _, existing := range a.observers {
			if existing == o {
				exists = true
				break
			}
		}
		if !exists {
			a.observers = append(a.observers, o)
		}
	}
}

// Deregister удаляет наблюдателя динамически.
//
// Незарегистрированный наблюдатель игнорируется.
func (a *AsyncPublisher) Deregister(observer Observer) error {
	if observer == nil {
		return nil
	}

	a.mu.Lock()
	var (
		found bool
		pos   int
	)
	for i, o := range a.observers {
		if o == observer {
			found = true
			pos = i
			break
		}
	}
	a.mu.Unlock()

	if found {
		a.observers = append(a.observers[:pos], a.observers[pos+1:]...)
		if o, ok := observer.(io.Closer); ok {
			return o.Close()
		}
	}
	return nil
}

// PublishLog публикует событие аудита для набора метрик.
func (a *AsyncPublisher) PublishLog(_ context.Context, now time.Time, ip string, metrics ...model.Metrics) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closing || len(a.observers) == 0 {
		return
	}

	ip = normalizeIP(ip)
	log := convertMetricsToAuditLog(now, ip, metrics)

	select {
	case a.queue <- log:
	default:
		// Не блокируем вызывающий код: при переполнении очереди логи отбрасываются.
		a.logger.Warn().
			Str("ip", ip).
			Int("metrics_count", len(metrics)).
			Msg("audit queue is full: dropping audit event")
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

// Run запускает фоновый воркер, отвечающий за уведомление заданных наблюдателей.
//
// Штатная остановка выполняется только отдельным вызовом Shutdown после завершения HTTP-серверов.
func (a *AsyncPublisher) Run() {
	ctx := context.Background()
	for {
		select {
		case <-a.shutdownCh:
			a.drainQueue(ctx)
			a.notifyWG.Wait()
			return
		case log := <-a.queue:
			a.notify(ctx, log)
		}
	}
}

func (a *AsyncPublisher) drainQueue(ctx context.Context) {
	for {
		select {
		case log := <-a.queue:
			a.notify(ctx, log)
		default:
			return
		}
	}
}

func (a *AsyncPublisher) notify(ctx context.Context, log model.AuditLog) {
	a.mu.Lock()
	observers := make([]Observer, len(a.observers))
	copy(observers, a.observers)
	a.mu.Unlock()

	for _, o := range observers {
		a.semaCh <- struct{}{}
		a.notifyWG.Add(1)
		go func(o Observer) {
			defer a.notifyWG.Done()
			defer func() {
				<-a.semaCh
			}()
			defer func() {
				// Паника одного наблюдателя не должна влиять на остальных наблюдателей.
				if r := recover(); r != nil {
					a.logger.Error().Interface("panic", r).Msg("audit observer panicked")
				}
			}()
			if err := o.update(ctx, log); err != nil {
				a.logger.Error().Err(err).Msg("audit observer update failed")
			}
		}(o)
	}
}

func (a *AsyncPublisher) Close() error {
	var errs []error
	for _, o := range a.observers {
		if o, ok := o.(io.Closer); ok {
			if err := o.Close(); err != nil {
				a.logger.Error().Msg(err.Error())
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (a *AsyncPublisher) Shutdown() {
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return
	}
	a.closing = true
	a.mu.Unlock()

	a.shutdownCh <- struct{}{}
}
