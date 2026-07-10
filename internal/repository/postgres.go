package repository

import (
	"context"
	"database/sql"
	"errors"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) (Repository, error) {
	repo := &PostgresRepository{db: db}
	return repo, nil
}

func (r *PostgresRepository) SaveBatch(ctx context.Context, urls []URLPair) (map[string]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO urls (id, original_url) 
        VALUES ($1, $2)
        ON CONFLICT (original_url) DO NOTHING
    `)
	if err != nil {
		return nil, err
	}

	for _, pair := range urls {
		if _, err := stmt.ExecContext(ctx, pair.ID, pair.URL); err != nil {
			stmt.Close()
			return nil, err
		}
	}
	stmt.Close()

	originalURLs := make([]string, len(urls))
	for i, pair := range urls {
		originalURLs[i] = pair.URL
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, original_url FROM urls WHERE original_url = ANY($1)`,
		pq.Array(originalURLs),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var id, url string
		if err := rows.Scan(&id, &url); err != nil {
			return nil, err
		}
		result[url] = id
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *PostgresRepository) Save(ctx context.Context, id, url string) error {
	query := `
		INSERT INTO urls (id, original_url) 
		VALUES ($1, $2)
		ON CONFLICT (original_url) DO NOTHING
	`
	res, err := r.db.ExecContext(ctx, query, id, url)
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrURLExists
	}

	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (string, bool) {
	var url string
	query := `SELECT original_url FROM urls WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&url)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false
		}
		return "", false
	}
	return url, true
}

func (r *PostgresRepository) FindByURL(ctx context.Context, url string) (string, bool) {
	var id string
	query := `SELECT id FROM urls WHERE original_url = $1`
	err := r.db.QueryRowContext(ctx, query, url).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false
		}
		return "", false
	}
	return id, true
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
