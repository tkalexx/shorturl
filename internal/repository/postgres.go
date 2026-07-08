package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) (Repository, error) {
	repo := &PostgresRepository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	return repo, nil
}

// миграция создаёт таблицы если их нет
func (r *PostgresRepository) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS urls (
		id VARCHAR(255) PRIMARY KEY,
		original_url TEXT NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);
	`
	_, err := r.db.Exec(query)
	return err
}

func (r *PostgresRepository) Save(ctx context.Context, id, url string) error {
	query := `
	INSERT INTO urls (id, original_url) 
	VALUES ($1, $2)
	ON CONFLICT (id) DO UPDATE SET original_url = EXCLUDED.original_url
	`
	_, err := r.db.ExecContext(ctx, query, id, url)
	return err
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
