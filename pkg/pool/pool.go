package pool

import "sync"

// Resetter описывает объекты, которые умеют сбрасывать своё состояние.
type Resetter interface {
	Reset()
}

// Pool хранит и повторно выдаёт объекты одного типа T.
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New создаёт пул объектов типа T.
func New[T Resetter](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFunc()
			},
		},
	}
}

// Get возвращает объект из пула.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put сбрасывает состояние объекта и помещает его обратно в пул.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.pool.Put(v)
}
