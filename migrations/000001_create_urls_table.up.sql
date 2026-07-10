CREATE TABLE IF NOT EXISTS urls (
                                    id VARCHAR(255) PRIMARY KEY,
    original_url TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

CREATE UNIQUE INDEX IF NOT EXISTS idx_original_url_unique ON urls(original_url);
