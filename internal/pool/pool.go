package pool

import "sync"

// Resetter описывает объекты, которые могут очищать свое внутреннее состояние перед повторным использованием.
type Resetter interface {
	Reset()
}

// Pool хранит переиспользуемые объекты одного типа, удовлетворяющего интерфейс Resetter.
// Порядок выдачи не гарантируется.
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New создает пустой пул для объектов, реализующих Resetter.
func New[T Resetter](newF func() T) *Pool[T] {
	return &(Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return newF()
			},
		},
	})
}

// Get возвращает последний сохраненный объект из пула или нулевое значение, если пул пуст.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put вызывает Reset для x и помещает его в пул для повторного использования.
func (p *Pool[T]) Put(x T) {
	x.Reset()
	p.pool.Put(x)
}
