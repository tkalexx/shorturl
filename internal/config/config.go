package config

import (
	"flag"
	"os"
	"strings"
)

// Config хранит настройки сервиса
type Config struct {
	RunAddr         string // адрес и порт
	BaseURL         string // базовый адрес сокращенных ссылок
	FileStoragePath string // путь к файлу хранения URL
	DatabaseDSN     string
}

// NewConfig инициализирует и парсит флаги командной строки
func NewConfig() *Config {
	return Parse(os.Args[1:])
}

// Parse разбирает аргументы и переменные окружения в Config.
func Parse(args []string) *Config {
	cfg := &Config{}

	fs := flag.NewFlagSet("shortener", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.RunAddr, "a", ":8080", "address and port to run server (e.g., localhost:8888)")
	fs.StringVar(&cfg.BaseURL, "b", "", "base URL for shortened links (e.g., http://localhost:8000/q)")
	fs.StringVar(&cfg.FileStoragePath, "file-storage-path", "/tmp/short-url-db.json", "path to file storage for URLs")
	fs.StringVar(&cfg.DatabaseDSN, "database-dsn", "", "database connection string")
	fs.StringVar(&cfg.DatabaseDSN, "d", "", "database connection string")
	_ = fs.Parse(args)

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
