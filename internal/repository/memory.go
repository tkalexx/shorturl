package repository

import (
	"context"
	"sync"
)

// InMemory — простое in-memory хранилище без сохранения на диск
type InMemory struct {
	mu   sync.RWMutex
	urls map[string]string
}

func NewInMemory() Repository {
	return &InMemory{
		urls: make(map[string]string),
	}
}

func (m *InMemory) SaveBatch(_ context.Context, urls []URLPair) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pair := range urls {
		m.urls[pair.ID] = pair.URL
	}
	return nil
}

func (m *InMemory) Save(_ context.Context, id, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.urls[id] = url
	return nil
}

func (m *InMemory) Get(_ context.Context, id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	url, ok := m.urls[id]
	return url, ok
}

func (m *InMemory) FindByURL(_ context.Context, url string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, u := range m.urls {
		if u == url {
			return id, true
		}
	}
	return "", false
}
