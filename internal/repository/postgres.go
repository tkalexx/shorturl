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

func (r *PostgresRepository) SaveBatch(ctx context.Context, urls []URLPair) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO urls (id, original_url) 
		VALUES ($1, $2)
		ON CONFLICT (original_url) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, pair := range urls {
		if _, err := stmt.ExecContext(ctx, pair.ID, pair.URL); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PostgresRepository) migrate() error {
	// Создаём таблицу и уникальный индекс на original_url
	query := `
	CREATE TABLE IF NOT EXISTS urls (
		id VARCHAR(255) PRIMARY KEY,
		original_url TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	-- Уникальный индекс для избежания дубликатов и race conditions
	CREATE UNIQUE INDEX IF NOT EXISTS idx_original_url_unique ON urls(original_url);
	`
	_, err := r.db.Exec(query)
	return err
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
