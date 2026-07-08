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

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(records)
}

func (r *FileRepository) Save(_ context.Context, id, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	r.storage[id] = &fileRecord{
		UUID:        fmt.Sprintf("%d", r.counter),
		ShortURL:    id,
		OriginalURL: url,
	}
	r.reverse[url] = id

	return r.save()
}

func (r *FileRepository) Get(_ context.Context, id string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.storage[id]
	if !ok {
		return "", false
	}
	return rec.OriginalURL, true
}

func (r *FileRepository) FindByURL(_ context.Context, url string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.reverse[url]
	return id, ok
}
