package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/soheilhy/cmux"
	"github.com/tkalexx/shorturl.git"
	"github.com/tkalexx/shorturl.git/internal/audit"
	"github.com/tkalexx/shorturl.git/internal/auth"
	"github.com/tkalexx/shorturl.git/internal/cert"
	"github.com/tkalexx/shorturl.git/internal/config"
	"github.com/tkalexx/shorturl.git/internal/db"
	"github.com/tkalexx/shorturl.git/internal/grpcserver"
	"github.com/tkalexx/shorturl.git/internal/handler"
	"github.com/tkalexx/shorturl.git/internal/logger"
	"github.com/tkalexx/shorturl.git/internal/repository"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const shutdownTimeout = 30 * time.Second

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	printBuildInfo()

	// обрабатываем аргументы командной строки
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

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

	listener, err := net.Listen("tcp", cfg.RunAddr)
	if err != nil {
		return err
	}
	if cfg.EnableHTTPS {
		tlsCert, err := cert.LoadOrGenerate(cert.CertFile, cert.KeyFile)
		if err != nil {
			return err
		}
		logger.Log.Info("HTTPS enabled",
			zap.String("cert", cert.CertFile),
			zap.String("key", cert.KeyFile),
		)
		listener = tls.NewListener(listener, &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			NextProtos:   []string{"h2", "http/1.1"},
		})
	}

	mux := cmux.New(listener)
	grpcL := mux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpL := mux.Match(cmux.Any())

	router := handler.NewRouter(service, authManager, auditor, cfg.TrustedSubnet)
	httpServer := &http.Server{Handler: router}
	grpcServer := grpcserver.NewGRPCServer(service, authManager, auditor)

	serverErr := make(chan error, 3)
	go func() {
		if err := grpcServer.Serve(grpcL); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverErr <- fmt.Errorf("grpc: %w", err)
		}
	}()
	go func() {
		if err := httpServer.Serve(httpL); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("http: %w", err)
		}
	}()
	go func() {
		if err := mux.Serve(); err != nil {
			// cmux returns error when listener is closed during shutdown
			if !errors.Is(err, net.ErrClosed) && !errors.Is(err, cmux.ErrListenerClosed) {
				serverErr <- fmt.Errorf("cmux: %w", err)
			}
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case err := <-serverErr:
		return err
	case sig := <-stop:
		logger.Log.Info("shutdown signal received", zap.String("signal", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-ctx.Done():
		grpcServer.Stop()
	}

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Log.Error("server shutdown failed", zap.Error(err))
	}
	_ = listener.Close()

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
