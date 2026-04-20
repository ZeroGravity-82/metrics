package pool

import "sync"

// Resetter описывает объекты, которые могут очищать свое внутреннее состояние перед повторным использованием.
type Resetter interface {
	Reset()
}

// Pool хранит переиспользуемые объекты и возвращает их в порядке LIFO.
type Pool[T Resetter] struct {
	items []T
	mu    sync.Mutex
}

// New создает пустой пул для объектов, реализующих Resetter.
func New[T Resetter]() *Pool[T] {
	return &(Pool[T]{})
}

// Get возвращает последний сохраненный объект из пула или нулевое значение, если пул пуст.
func (p *Pool[T]) Get() T {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := len(p.items)
	if l <= 0 {
		var zero T
		return zero
	}
	i := p.items[l-1]
	p.items = p.items[:l-1]
	return i
}

// Put вызывает Reset для x и помещает его в пул для повторного использования.
func (p *Pool[T]) Put(x T) {
	p.mu.Lock()
	defer p.mu.Unlock()
	x.Reset()
	p.items = append(p.items, x)
}
