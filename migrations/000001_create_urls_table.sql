CREATE TABLE IF NOT EXISTS urls (
                                    id VARCHAR(255) PRIMARY KEY,
    original_url TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

DROP TABLE IF EXISTS urls;
