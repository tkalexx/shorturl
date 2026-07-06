package main

import (
	"net/http"

	"github.com/tkalexx/shorturl.git/internal/config"
	"github.com/tkalexx/shorturl.git/internal/handler"
	"github.com/tkalexx/shorturl.git/internal/logger"
	"github.com/tkalexx/shorturl.git/internal/repository"
	"go.uber.org/zap"
)

func main() {
	// обрабатываем аргументы командной строки
	cfg := config.NewConfig()

	if err := run(cfg); err != nil {
		panic(err)
	}
}

func run(cfg *config.Config) error {
	if err := logger.Initialize("info"); err != nil {
		return err
	}

	// Выбираем тип хранилища в зависимости от конфигурации
	var repo repository.Repository
	if cfg.FileStoragePath != "" {
		// Файловое хранилище с сохранением на диск
		fileRepo, err := repository.NewFileRepository(cfg.FileStoragePath)
		if err != nil {
			logger.Log.Fatal("Failed to initialize file storage", zap.Error(err))
		}
		repo = fileRepo
		logger.Log.Info("Using file storage", zap.String("path", cfg.FileStoragePath))
	} else {
		// In-memory хранилище
		repo = repository.NewInMemory()
		logger.Log.Info("Using in-memory storage")
	}

	service := handler.NewService(repo)
	service.SetBaseURL(cfg.BaseURL)

	logger.Log.Info("Running server",
		zap.String("address", cfg.RunAddr),
		zap.String("base_url", cfg.BaseURL),
	)

	return http.ListenAndServe(cfg.RunAddr, handler.NewRouter(service))
}
