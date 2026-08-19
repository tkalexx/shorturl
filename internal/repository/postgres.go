package repository

import (
	"context"
	"database/sql"
	"errors"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
)

// PostgresRepository хранит короткие ссылки в PostgreSQL.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository создаёт репозиторий поверх открытого соединения с БД.
func NewPostgresRepository(db *sql.DB) (Repository, error) {
	repo := &PostgresRepository{db: db}
	return repo, nil
}

// SaveBatch пакетно сохраняет URL в одной транзакции.
func (r *PostgresRepository) SaveBatch(ctx context.Context, urls []URLPair) (map[string]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO urls (id, original_url, user_id) 
        VALUES ($1, $2, $3)
        ON CONFLICT (original_url) DO NOTHING
    `)
	if err != nil {
		return nil, err
	}

	for _, pair := range urls {
		if _, err := stmt.ExecContext(ctx, pair.ID, pair.URL, pair.UserID); err != nil {
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

// Save сохраняет URL; при конфликте оригинального адреса возвращает ErrURLExists.
func (r *PostgresRepository) Save(ctx context.Context, id, url, userID string) error {
	query := `
		INSERT INTO urls (id, original_url, user_id) 
		VALUES ($1, $2, $3)
		ON CONFLICT (original_url) DO NOTHING
	`
	res, err := r.db.ExecContext(ctx, query, id, url, userID)
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrURLExists
	}

	return nil
}

// Get возвращает оригинальный URL, флаг удаления и признак наличия записи.
func (r *PostgresRepository) Get(ctx context.Context, id string) (string, bool, bool) {
	var url string
	var deleted bool
	query := `SELECT original_url, is_deleted FROM urls WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&url, &deleted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, false
		}
		return "", false, false
	}
	return url, deleted, true
}

// FindByURL ищет короткий идентификатор по оригинальному URL.
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

// GetByUserID возвращает все неудалённые URL пользователя.
func (r *PostgresRepository) GetByUserID(ctx context.Context, userID string) ([]UserURL, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, original_url FROM urls WHERE user_id = $1 AND is_deleted = FALSE`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]UserURL, 0)
	for rows.Next() {
		var u UserURL
		if err := rows.Scan(&u.ID, &u.OriginalURL); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// MarkDeleted помечает URL пользователя как удалённые.
func (r *PostgresRepository) MarkDeleted(ctx context.Context, ids []string, userID string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE urls
		SET is_deleted = TRUE
		WHERE id = ANY($1) AND user_id = $2 AND is_deleted = FALSE
	`, pq.Array(ids), userID)
	return err
}

// Stats возвращает число URL и уникальных пользователей.
func (r *PostgresRepository) Stats(ctx context.Context) (int, int, error) {
	var urls int
	var users int
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS urls_count,
			COUNT(DISTINCT NULLIF(user_id, '')) AS users_count
		FROM urls
	`).Scan(&urls, &users)
	if err != nil {
		return 0, 0, err
	}
	return urls, users, nil
}

// Ping проверяет соединение с PostgreSQL.
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
