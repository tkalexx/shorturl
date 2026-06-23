package config

import "flag"

// Config хранит настройки сервиса
type Config struct {
	RunAddr string // адрес и порт для запуска сервера
	BaseURL string // базовый адрес сокращенных ссылок
}

// NewConfig инициализирует и парсит флаги командной строки
func NewConfig() *Config {
	cfg := &Config{}

	// Регистрируем флаги
	flag.StringVar(&cfg.RunAddr, "a", ":8080", "address and port to run server (e.g., localhost:8888)")
	flag.StringVar(&cfg.BaseURL, "b", "", "base URL for shortened links (e.g., http://localhost:8000/q)")

	// Парсим флаги
	flag.Parse()

	return cfg
}
