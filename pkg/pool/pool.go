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
	p := &Pool[T]{}
	if newFunc != nil {
		p.pool.New = func() any {
			return newFunc()
		}
	}
	return p
}

// Get возвращает объект из пула.
func (p *Pool[T]) Get() T {
	v := p.pool.Get()
	if v == nil {
		var zero T
		return zero
	}
	return v.(T)
}

// Put сбрасывает состояние объекта и помещает его обратно в пул.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.pool.Put(v)
}
