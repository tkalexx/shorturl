package repository

import "sync"

type InMemory struct {
	mu   sync.RWMutex
	urls map[string]string
}

func NewInMemory() Repository {
	return &InMemory{
		urls: make(map[string]string),
	}
}

func (m *InMemory) Save(id, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.urls[id] = url
	return nil
}

func (m *InMemory) Get(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	url, ok := m.urls[id]
	return url, ok
}

func (m *InMemory) FindByURL(url string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, u := range m.urls {
		if u == url {
			return id, true
		}
	}
	return "", false
}
