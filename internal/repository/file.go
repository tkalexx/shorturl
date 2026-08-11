package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// fileRecord структура для JSON-файла
type fileRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
	Deleted     bool   `json:"is_deleted,omitempty"`
}

// FileRepository хранит данные в памяти и сохраняет в файл
type FileRepository struct {
	mu      sync.RWMutex
	storage map[string]*fileRecord // shortID -> record
	reverse map[string]string      // originalURL -> shortID
	path    string
	counter int
}

// NewFileRepository создаёт репозиторий с загрузкой из файла
func NewFileRepository(path string) (Repository, error) {
	repo := &FileRepository{
		storage: make(map[string]*fileRecord),
		reverse: make(map[string]string),
		path:    path,
		counter: 0,
	}

	if err := repo.load(); err != nil {
		return nil, err
	}
	return repo, nil
}

// load загружает данные из файла при старте
func (r *FileRepository) load() error {
	file, err := os.Open(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	var records []fileRecord
	if err := json.NewDecoder(file).Decode(&records); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	for _, rec := range records {
		r.storage[rec.ShortURL] = &rec
		r.reverse[rec.OriginalURL] = rec.ShortURL

		var num int
		fmt.Sscanf(rec.UUID, "%d", &num)
		if num > r.counter {
			r.counter = num
		}
	}
	return nil
}

// save синхронизирует данные с файлом
func (r *FileRepository) save() error {
	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	records := make([]fileRecord, 0, len(r.storage))
	for _, rec := range r.storage {
		records = append(records, *rec)
	}

	return json.NewEncoder(file).Encode(records)
}

// SaveBatch сохраняет пакет URL и синхронизирует файл.
func (r *FileRepository) SaveBatch(_ context.Context, urls []URLPair) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make(map[string]string) // originalURL -> shortID

	for _, pair := range urls {
		r.counter++
		r.storage[pair.ID] = &fileRecord{
			UUID:        fmt.Sprintf("%d", r.counter),
			ShortURL:    pair.ID,
			OriginalURL: pair.URL,
			UserID:      pair.UserID,
		}
		r.reverse[pair.URL] = pair.ID
		result[pair.URL] = pair.ID
	}

	if err := r.save(); err != nil {
		return nil, err
	}

	return result, nil
}

// Save сохраняет URL и записывает изменения в файл.
func (r *FileRepository) Save(_ context.Context, id, url, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existingID, ok := r.reverse[url]; ok && existingID != id {
		return ErrURLExists
	}

	r.counter++
	r.storage[id] = &fileRecord{
		UUID:        fmt.Sprintf("%d", r.counter),
		ShortURL:    id,
		OriginalURL: url,
		UserID:      userID,
	}
	r.reverse[url] = id

	return r.save()
}

// Get возвращает оригинальный URL, флаг удаления и признак наличия записи.
func (r *FileRepository) Get(_ context.Context, id string) (string, bool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.storage[id]
	if !ok {
		return "", false, false
	}
	return rec.OriginalURL, rec.Deleted, true
}

// FindByURL ищет короткий идентификатор по оригинальному URL.
func (r *FileRepository) FindByURL(_ context.Context, url string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.reverse[url]
	return id, ok
}

// GetByUserID возвращает все неудалённые URL пользователя.
func (r *FileRepository) GetByUserID(_ context.Context, userID string) ([]UserURL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]UserURL, 0)
	for _, rec := range r.storage {
		if rec.UserID == userID && !rec.Deleted {
			result = append(result, UserURL{ID: rec.ShortURL, OriginalURL: rec.OriginalURL})
		}
	}
	return result, nil
}

// MarkDeleted помечает URL пользователя как удалённые и сохраняет файл.
func (r *FileRepository) MarkDeleted(_ context.Context, ids []string, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	changed := false
	for _, id := range ids {
		rec, ok := r.storage[id]
		if !ok || rec.UserID != userID {
			continue
		}
		rec.Deleted = true
		changed = true
	}
	if !changed {
		return nil
	}
	return r.save()
}

// Stats возвращает число URL и уникальных пользователей.
func (r *FileRepository) Stats(_ context.Context) (int, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make(map[string]struct{})
	for _, rec := range r.storage {
		if rec.UserID != "" {
			users[rec.UserID] = struct{}{}
		}
	}
	return len(r.storage), len(users), nil
}

// Ping проверяет возможность записи в файл хранилища.
func (r *FileRepository) Ping(_ context.Context) error {
	if r.path == "" {
		return fmt.Errorf("file path not set")
	}

	file, err := os.OpenFile(r.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}
