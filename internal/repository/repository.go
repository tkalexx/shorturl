package repository

import (
	"context"
	"errors"
)

var (
	ErrURLExists = errors.New("url already exists")
)

type URLPair struct {
	ID  string
	URL string
}

type Repository interface {
	Save(ctx context.Context, id, url string) error
	SaveBatch(ctx context.Context, urls []URLPair) error
	Get(ctx context.Context, id string) (url string, found bool)
	FindByURL(ctx context.Context, url string) (id string, found bool)
}
