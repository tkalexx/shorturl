package repository

import (
	"context"
	"errors"
)

var ErrURLExists = errors.New("url already exists")

type URLPair struct {
	ID     string
	URL    string
	UserID string
}

// UserURL - пара short ID и оригинального URL для конкретного пользователя.
type UserURL struct {
	ID          string
	OriginalURL string
}

type Repository interface {
	Save(ctx context.Context, id, url, userID string) error
	SaveBatch(ctx context.Context, urls []URLPair) (map[string]string, error)
	Get(ctx context.Context, id string) (url string, deleted bool, found bool)
	FindByURL(ctx context.Context, url string) (id string, found bool)
	GetByUserID(ctx context.Context, userID string) ([]UserURL, error)
	MarkDeleted(ctx context.Context, ids []string, userID string) error
	Ping(ctx context.Context) error
}
