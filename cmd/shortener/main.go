package main

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"

	"github.com/pressly/goose/v3"
	"github.com/tkalexx/shorturl.git"
	"github.com/tkalexx/shorturl.git/internal/audit"
	"github.com/tkalexx/shorturl.git/internal/auth"
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

	if err := cfg.Validate(); err != nil {
		return err
	}

	authManager, err := auth.NewManager(cfg.AuthSecret)
	if err != nil {
		return err
	}

	var (
		repo   repository.Repository
		dbConn *sql.DB
	)
	switch {
	case cfg.DatabaseDSN != "":
		var err error
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

	auditor := audit.BuildFromConfig(cfg.AuditFile, cfg.AuditURL)

	logger.Log.Info("Running server",
		zap.String("address", cfg.RunAddr),
		zap.String("base_url", cfg.BaseURL),
		zap.String("audit_file", cfg.AuditFile),
		zap.String("audit_url", cfg.AuditURL),
	)

	return http.ListenAndServe(cfg.RunAddr, handler.NewRouter(service, authManager, auditor))
}

func runMigrations(db *sql.DB) error {
	migrationsFS, err := fs.Sub(shorturl.MigrationsFS, "migrations")
	if err != nil {
		return err
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrationsFS,
	)
	if err != nil {
		return err
	}

	_, err = provider.Up(context.Background())
	return err
}
