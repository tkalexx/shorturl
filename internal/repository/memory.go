package repository

import (
	"context"
	"sync"
)

// InMemory — простое in-memory хранилище без сохранения на диск
type InMemory struct {
	mu      sync.RWMutex
	urls    map[string]string
	reverse map[string]string // url -> id
}

func NewInMemory() Repository {
	return &InMemory{
		urls:    make(map[string]string),
		reverse: make(map[string]string),
	}
}

func (m *InMemory) SaveBatch(_ context.Context, urls []URLPair) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pair := range urls {
		m.urls[pair.ID] = pair.URL
		m.reverse[pair.URL] = pair.ID
	}
	return nil
}

func (m *InMemory) Save(_ context.Context, id, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existingID, ok := m.reverse[url]; ok && existingID != id {
		return ErrURLExists
	}

	m.urls[id] = url
	m.reverse[url] = id
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
	id, ok := m.reverse[url]
	return id, ok
}
