package main

import (
	"database/sql"
	"net/http"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
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
	switch {
	case cfg.DatabaseDSN != "":
		dbConn, err = db.NewDB(cfg.DatabaseDSN)
		if err != nil {
			logger.Log.Fatal("Failed to connect to database", zap.Error(err))
		}
		defer dbConn.Close()

		if err := runMigrations(dbConn); err != nil {
			logger.Log.Fatal("Failed to run migrations", zap.Error(err))
		}

		pgRepo, err := repository.NewPostgresRepository(dbConn)
		if err != nil {
			logger.Log.Fatal("Failed to initialize postgres repository", zap.Error(err))
		}
		repo = pgRepo
		logger.Log.Info("Using PostgreSQL storage")
	case cfg.FileStoragePath != "":
		fileRepo, err := repository.NewFileRepository(cfg.FileStoragePath)
		if err != nil {
			logger.Log.Fatal("Failed to initialize file storage", zap.Error(err))
		}
		repo = fileRepo
		logger.Log.Info("Using file storage", zap.String("path", cfg.FileStoragePath))
	default:
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

func runMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres",
		driver,
	)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
