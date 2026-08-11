package repository

import (
	"context"
	"sync"
)

type memoryRecord struct {
	URL     string
	UserID  string
	Deleted bool
}

// InMemory — простое in-memory хранилище без сохранения на диск
type InMemory struct {
	mu      sync.RWMutex
	urls    map[string]memoryRecord
	reverse map[string]string // url -> id
}

// NewInMemory создаёт пустое in-memory хранилище.
func NewInMemory() Repository {
	return &InMemory{
		urls:    make(map[string]memoryRecord),
		reverse: make(map[string]string),
	}
}

// SaveBatch сохраняет пакет URL без проверки уникальности.
func (m *InMemory) SaveBatch(ctx context.Context, urls []URLPair) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]string)
	for _, pair := range urls {
		m.urls[pair.ID] = memoryRecord{URL: pair.URL, UserID: pair.UserID}
		m.reverse[pair.URL] = pair.ID
		result[pair.URL] = pair.ID
	}
	return result, nil
}

// Ping всегда успешен для in-memory хранилища.
func (m *InMemory) Ping(ctx context.Context) error {
	return nil
}

// Save сохраняет пару id→url; при конфликте оригинального URL возвращает ErrURLExists.
func (m *InMemory) Save(_ context.Context, id, url, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existingID, ok := m.reverse[url]; ok && existingID != id {
		return ErrURLExists
	}

	m.urls[id] = memoryRecord{URL: url, UserID: userID}
	m.reverse[url] = id
	return nil
}

// Get возвращает оригинальный URL, флаг удаления и признак наличия записи.
func (m *InMemory) Get(_ context.Context, id string) (string, bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.urls[id]
	if !ok {
		return "", false, false
	}
	return rec.URL, rec.Deleted, true
}

// FindByURL ищет короткий идентификатор по оригинальному URL.
func (m *InMemory) FindByURL(_ context.Context, url string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.reverse[url]
	return id, ok
}

// GetByUserID возвращает все неудалённые URL пользователя.
func (m *InMemory) GetByUserID(_ context.Context, userID string) ([]UserURL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]UserURL, 0)
	for id, rec := range m.urls {
		if rec.UserID == userID && !rec.Deleted {
			result = append(result, UserURL{ID: id, OriginalURL: rec.URL})
		}
	}
	return result, nil
}

// MarkDeleted помечает URL пользователя как удалённые.
func (m *InMemory) MarkDeleted(_ context.Context, ids []string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		rec, ok := m.urls[id]
		if !ok || rec.UserID != userID {
			continue
		}
		rec.Deleted = true
		m.urls[id] = rec
	}
	return nil
}
