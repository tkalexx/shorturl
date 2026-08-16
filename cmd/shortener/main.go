package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"
	"os"

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

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	printBuildInfo()

	// обрабатываем аргументы командной строки
	cfg := config.NewConfig()

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
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
	defer auditor.Close()

	if cfg.PprofAddr != "" {
		go func() {
			logger.Log.Info("Starting pprof server", zap.String("address", cfg.PprofAddr))
			if err := http.ListenAndServe(cfg.PprofAddr, handler.NewPprofRouter()); err != nil {
				logger.Log.Error("pprof server stopped", zap.Error(err))
			}
		}()
	}

	logger.Log.Info("Running server",
		zap.String("address", cfg.RunAddr),
		zap.String("base_url", cfg.BaseURL),
		zap.String("audit_file", cfg.AuditFile),
		zap.String("audit_url", cfg.AuditURL),
		zap.String("pprof_addr", cfg.PprofAddr),
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
