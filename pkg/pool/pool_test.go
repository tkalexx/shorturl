package pool

import "testing"

type sample struct {
	n    int
	data []byte
}

func (s *sample) Reset() {
	s.n = 0
	s.data = s.data[:0]
}

func TestPoolGetPutResets(t *testing.T) {
	p := New(func() *sample {
		return &sample{data: make([]byte, 0, 8)}
	})

	obj := p.Get()
	obj.n = 42
	obj.data = append(obj.data, 'a', 'b', 'c')
	p.Put(obj)

	reused := p.Get()
	if reused.n != 0 {
		t.Fatalf("n = %d, want 0 after Reset", reused.n)
	}
	if len(reused.data) != 0 {
		t.Fatalf("len(data) = %d, want 0 after Reset", len(reused.data))
	}
	if cap(reused.data) == 0 {
		t.Fatal("expected slice capacity to be preserved")
	}
}
