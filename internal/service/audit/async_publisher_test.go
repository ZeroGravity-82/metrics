package audit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"zerogravity-82/metrics/internal/model"
)

type testObserver struct {
	calls atomic.Int64
	wg    *sync.WaitGroup
}

func (o *testObserver) update(_ context.Context, _ model.AuditLog) error {
	o.calls.Add(1)
	if o.wg != nil {
		o.wg.Done()
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
	p.Deregister(o1)

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
	p.Deregister(o4)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Run(ctx)

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
