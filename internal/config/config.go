package config

import (
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/tkalexx/shorturl.git/internal/auth"
)

// Config хранит настройки сервиса.
type Config struct {
	RunAddr         string // адрес и порт
	BaseURL         string // базовый адрес сокращенных ссылок
	FileStoragePath string // путь к файлу хранения URL
	DatabaseDSN     string // строка подключения к PostgreSQL
	AuthSecret      string // секрет подписи auth-cookie
	AuditFile       string // путь к файлу аудита; пусто - аудит в файл отключён
	AuditURL        string // URL удалённого приёмника аудита; пусто - отключён
}

// NewConfig инициализирует и парсит флаги командной строки
func NewConfig() *Config {
	cfg := &Config{}

	// Регистрируем флаги
	flag.StringVar(&cfg.RunAddr, "a", ":8080", "address and port to run server (e.g., localhost:8888)")
	flag.StringVar(&cfg.BaseURL, "b", "", "base URL for shortened links (e.g., http://localhost:8000/q)")
	flag.StringVar(&cfg.FileStoragePath, "file-storage-path", "/tmp/short-url-db.json", "path to file storage for URLs")
	flag.StringVar(&cfg.DatabaseDSN, "database-dsn", "", "database connection string")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database connection string")
	flag.StringVar(&cfg.AuthSecret, "auth-secret", "", "secret key for signing auth cookies")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "path to audit log file")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "remote audit receiver URL")
	flag.Parse()

	if envAddr := os.Getenv("SERVER_ADDRESS"); envAddr != "" {
		cfg.RunAddr = envAddr
	}

	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}

	if envFilePath := os.Getenv("FILE_STORAGE_PATH"); envFilePath != "" {
		cfg.FileStoragePath = envFilePath
	}

	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		cfg.DatabaseDSN = envDSN
	}

	if envSecret := os.Getenv("AUTH_SECRET"); envSecret != "" {
		cfg.AuthSecret = envSecret
	}

	if envAuditFile := os.Getenv("AUDIT_FILE"); envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	}

	if envAuditURL := os.Getenv("AUDIT_URL"); envAuditURL != "" {
		cfg.AuditURL = envAuditURL
	}

	if cfg.FileStoragePath == "" {
		cfg.FileStoragePath = "/tmp/short-url-db.json"
	}

	if cfg.BaseURL == "" {
		addr := cfg.RunAddr
		if strings.HasPrefix(addr, ":") {
			addr = "localhost" + addr
		}
		cfg.BaseURL = "http://" + addr
	}

	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")

	return cfg
}

// Validate проверяет обязательные настройки сервиса.
func (c *Config) Validate() error {
	if _, err := auth.NewManager(c.AuthSecret); err != nil {
		return errors.New("invalid auth secret: " + err.Error())
	}
	return nil
}
