package main

import (
	"database/sql"
	"net/http"

	"github.com/tkalexx/shorturl.git/internal/config"
	"github.com/tkalexx/shorturl.git/internal/db"
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

	var (
		repo   repository.Repository
		dbConn *sql.DB
		err    error
	)

	if cfg.DatabaseDSN != "" {
		dbConn, err = db.NewDB(cfg.DatabaseDSN)
		if err != nil {
			logger.Log.Fatal("Failed to connect to database", zap.Error(err))
		}
		defer dbConn.Close()
		logger.Log.Info("Connected to database", zap.String("dsn", cfg.DatabaseDSN))
	}

	// Выбираем тип хранилища в зависимости от конфигурации
	if cfg.FileStoragePath != "" && cfg.DatabaseDSN == "" {
		// Файловое хранилище с сохранением на диск
		fileRepo, err := repository.NewFileRepository(cfg.FileStoragePath)
		if err != nil {
			logger.Log.Fatal("Failed to initialize file storage", zap.Error(err))
		}
		repo = fileRepo
		logger.Log.Info("Using file storage", zap.String("path", cfg.FileStoragePath))
	} else if cfg.DatabaseDSN == "" {
		// In-memory хранилище
		repo = repository.NewInMemory()
		logger.Log.Info("Using in-memory storage")
	} else {
		// Когда нужно использовать БД как основное хранилище:
		// repo = repository.NewDBRepository(dbConn)
		repo = repository.NewInMemory()
		logger.Log.Info("Using in-memory storage (DB connected for ping only)")
	}

	service := handler.NewService(repo, dbConn)
	service.SetBaseURL(cfg.BaseURL)

	logger.Log.Info("Running server",
		zap.String("address", cfg.RunAddr),
		zap.String("base_url", cfg.BaseURL),
	)

	return http.ListenAndServe(cfg.RunAddr, handler.NewRouter(service))
}
