package audit

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/model"
)

type testObserver struct {
	calls atomic.Int64
	wg    *sync.WaitGroup
	fn    func(context.Context, model.AuditLog) error
}

func (o *testObserver) update(ctx context.Context, log model.AuditLog) error {
	o.calls.Add(1)
	if o.wg != nil {
		o.wg.Done()
	}
	if o.fn != nil {
		return o.fn(ctx, log)
	}
	return nil
}

// TestAsyncPublisher_Register_DedupAndNil проверяет, что Register() игнорирует nil и не добавляет повторно один и тот
// же экземпляр наблюдателя.
func TestAsyncPublisher_NewAsyncPublisher_DedupAndNil(t *testing.T) {
	// Act
	o1 := &testObserver{}
	o2 := &testObserver{}
	p := NewAsyncPublisher(zerolog.Nop(), o1, o2, nil, o1)

	// Assert
	assert.Equal(t, 2, len(p.observers))
	assert.Same(t, o1, p.observers[0])
	assert.Same(t, o2, p.observers[1])
}

// TestAsyncPublisher_Register_DedupAndNil проверяет, что Register() игнорирует nil и не добавляет повторно один и тот
// же экземпляр наблюдателя.
func TestAsyncPublisher_Register_DedupAndNil(t *testing.T) {
	// Arrange
	o1 := &testObserver{}
	p := NewAsyncPublisher(zerolog.Nop(), o1)
	o2 := &testObserver{}

	// Act
	p.Register(nil, o1, o2)

	// Assert
	assert.Equal(t, 2, len(p.observers))
	assert.Same(t, o1, p.observers[0])
	assert.Same(t, o2, p.observers[1])
}

// TestAsyncPublisher_Deregister_Removes проверяет, что Deregister() удаляет наблюдателя и сохраняет остальных без
// изменения порядка.
func TestAsyncPublisher_Deregister_Removes(t *testing.T) {
	// Arrange
	o1 := &testObserver{}
	p := NewAsyncPublisher(zerolog.Nop(), o1)
	o2 := &testObserver{}
	o3 := &testObserver{}
	p.Register(o2, o3)

	// Act
	_ = p.Deregister(o1)

	// Assert
	assert.Len(t, p.observers, 2)
	assert.Same(t, o2, p.observers[0])
	assert.Same(t, o3, p.observers[1])
}

// TestAsyncPublisher_PublishLog_NotifyObserversSuccessfully проверяет, что PublishLog() успешно уведомляет всех
// зарегистрированных наблюдателей о новом событии аудита.
func TestAsyncPublisher_PublishLog_NotifyObserversSuccessfully(t *testing.T) {
	// Arrange
	var wg sync.WaitGroup
	wg.Add(3)

	o1 := &testObserver{wg: &wg}
	o2 := &testObserver{wg: &wg}
	o3 := &testObserver{wg: &wg}
	o4 := &testObserver{wg: &wg}
	p := NewAsyncPublisher(zerolog.Nop(), o1, o4)
	p.Register(o2, o3)
	_ = p.Deregister(o4)

	go p.Run()

	// Act
	p.PublishLog(context.Background(), time.Now(), "127.0.0.1:1234", model.Metrics{ID: "m1"})

	// Assert
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()

	select {
	case <-ch:
		// Expected path
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for observers")
	}

	// Keep original observers assertions too.
	assert.EqualValues(t, 1, o1.calls.Load())
	assert.EqualValues(t, 1, o2.calls.Load())
	assert.EqualValues(t, 1, o3.calls.Load())
	assert.Zero(t, o4.calls.Load())
}

// TestAsyncPublisher_PublishLog_DropsWhenQueueFull проверяет, что при заполненной очереди PublishLog() не блокирует
// вызывающий код и не увеличивает размер очереди.
func TestAsyncPublisher_PublishLog_DropsWhenQueueFull(t *testing.T) {
	// Arrange
	p := NewAsyncPublisher(zerolog.Nop())
	p.Register(&testObserver{}) // должен быть зарегистрирован хотя бы один наблюдатель

	for i := 0; i < defaultQueueSize; i++ {
		p.queue <- model.AuditLog{TS: int64(i)}
	}

	// Act
	p.PublishLog(context.Background(), time.Now(), "127.0.0.1:1234", model.Metrics{ID: "m1"})

	// Assert
	assert.Equal(t, defaultQueueSize, len(p.queue))
}

// TestAsyncPublisher_PublishLog_IgnoresEventsAfterShutdownStarts проверяет, что после начала Shutdown() новые события
// больше не принимаются в очередь публикации.
func TestAsyncPublisher_PublishLog_IgnoresEventsAfterShutdownStarts(t *testing.T) {
	// Arrange
	p := NewAsyncPublisher(zerolog.Nop(), &testObserver{})
	go p.Run()

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		p.Shutdown()
	}()

	// Arrange
	require.Eventually(t, func() bool { // Дожидаемся момента, когда паблишер уже вошел в режим остановки.
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.closing
	}, time.Second, 10*time.Millisecond)

	// Act
	p.PublishLog(context.Background(), time.Now(), "127.0.0.1:1234", model.Metrics{ID: "m1"})

	// Assert
	assert.Zero(t, len(p.queue))
	select { // Shutdown должен завершиться без зависания.
	case <-shutdownDone:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish")
	}
}

// TestAsyncPublisher_Shutdown_DrainsQueueAndWaitsForObservers проверяет, что Shutdown() дочитывает все накопленные
// события из очереди и дожидается завершения их доставки наблюдателям.
func TestAsyncPublisher_Shutdown_DrainsQueueAndWaitsForObservers(t *testing.T) {
	// Arrange
	processed := make(chan int64, 2)
	observer := &testObserver{ // Наблюдатель, который проверяет, что при drain используется рабочий контекст.
		fn: func(ctx context.Context, log model.AuditLog) error {
			if ctx.Err() != nil {
				return errors.New("shutdown context canceled before delivery")
			}
			processed <- log.TS
			return nil
		},
	}
	p := NewAsyncPublisher(zerolog.Nop(), observer)

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		p.Run()
	}()

	p.PublishLog(context.Background(), time.Unix(1, 0), "127.0.0.1:1234", model.Metrics{ID: "m1"})
	p.PublishLog(context.Background(), time.Unix(2, 0), "127.0.0.1:1234", model.Metrics{ID: "m2"})

	// Act
	p.Shutdown()

	// Assert
	received := map[int64]struct{}{}
	deadline := time.After(2 * time.Second)
	for len(received) < 2 { // Оба события должны быть вычитаны из очереди и доставлены наблюдателю.
		select {
		case ts := <-processed:
			received[ts] = struct{}{}
		case <-deadline:
			t.Fatal("timed out waiting for drained audit events")
		}
	}

	select { // После завершения drain фоновой воркер должен корректно остановиться.
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher did not stop after shutdown")
	}
}
