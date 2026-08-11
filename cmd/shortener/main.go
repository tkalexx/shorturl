package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/tkalexx/shorturl.git"
	"github.com/tkalexx/shorturl.git/internal/audit"
	"github.com/tkalexx/shorturl.git/internal/auth"
	"github.com/tkalexx/shorturl.git/internal/cert"
	"github.com/tkalexx/shorturl.git/internal/config"
	"github.com/tkalexx/shorturl.git/internal/db"
	"github.com/tkalexx/shorturl.git/internal/handler"
	"github.com/tkalexx/shorturl.git/internal/logger"
	"github.com/tkalexx/shorturl.git/internal/repository"
	"go.uber.org/zap"
)

const shutdownTimeout = 30 * time.Second

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	printBuildInfo()

	// обрабатываем аргументы командной строки
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func printBuildInfo() {
	fmt.Printf("Build version: %s\n", valueOrNA(buildVersion))
	fmt.Printf("Build date: %s\n", valueOrNA(buildDate))
	fmt.Printf("Build commit: %s\n", valueOrNA(buildCommit))
}

func valueOrNA(v string) string {
	if v == "" {
		return "N/A"
	}
	return v
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
	defer service.Close()

	auditor := audit.BuildFromConfig(cfg.AuditFile, cfg.AuditURL)
	defer auditor.Close()

	var pprofServer *http.Server
	if cfg.PprofAddr != "" {
		pprofServer = &http.Server{
			Addr:    cfg.PprofAddr,
			Handler: handler.NewPprofRouter(),
		}
		go func() {
			logger.Log.Info("Starting pprof server", zap.String("address", cfg.PprofAddr))
			if err := pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
		zap.Bool("enable_https", cfg.EnableHTTPS),
		zap.String("trusted_subnet", cfg.TrustedSubnet),
	)

	router := handler.NewRouter(service, authManager, auditor, cfg.TrustedSubnet)
	server := &http.Server{
		Addr:    cfg.RunAddr,
		Handler: router,
	}

	serverErr := make(chan error, 1)
	go func() {
		var err error
		if cfg.EnableHTTPS {
			if err = cert.EnsureFiles(cert.CertFile, cert.KeyFile); err != nil {
				serverErr <- err
				return
			}
			logger.Log.Info("HTTPS enabled",
				zap.String("cert", cert.CertFile),
				zap.String("key", cert.KeyFile),
			)
			err = server.ListenAndServeTLS(cert.CertFile, cert.KeyFile)
		} else {
			err = server.ListenAndServe()
		}
		serverErr <- err
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case sig := <-stop:
		logger.Log.Info("shutdown signal received", zap.String("signal", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error("server shutdown failed", zap.Error(err))
	}
	if pprofServer != nil {
		if err := pprofServer.Shutdown(ctx); err != nil {
			logger.Log.Error("pprof shutdown failed", zap.Error(err))
		}
	}

	logger.Log.Info("server stopped gracefully")
	return nil
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
