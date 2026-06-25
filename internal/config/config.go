package config

import (
	"flag"
	"os"
	"strings"
)

// Config хранит настройки сервиса
type Config struct {
	RunAddr string // адрес и порт
	BaseURL string // базовый адрес сокращенных ссылок
}

// NewConfig инициализирует и парсит флаги командной строки
func NewConfig() *Config {
	cfg := &Config{}

	// Регистрируем флаги
	flag.StringVar(&cfg.RunAddr, "a", ":8080", "address and port to run server (e.g., localhost:8888)")
	flag.StringVar(&cfg.BaseURL, "b", "", "base URL for shortened links (e.g., http://localhost:8000/q)")
	flag.Parse()

	if envAddr := os.Getenv("SERVER_ADDRESS"); envAddr != "" {
		cfg.RunAddr = envAddr
	}

	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
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
