package repository

import "context"

type Repository interface {
	Save(ctx context.Context, id, url string) error
	Get(ctx context.Context, id string) (url string, found bool)
	FindByURL(ctx context.Context, url string) (id string, found bool)
}
