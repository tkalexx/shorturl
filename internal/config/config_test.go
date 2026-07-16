package config

import (
	"os"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	t.Setenv("SERVER_ADDRESS", "")
	t.Setenv("BASE_URL", "")
	t.Setenv("FILE_STORAGE_PATH", "")
	t.Setenv("DATABASE_DSN", "")
	// clear env explicitly for isolation
	for _, k := range []string{"SERVER_ADDRESS", "BASE_URL", "FILE_STORAGE_PATH", "DATABASE_DSN"} {
		_ = os.Unsetenv(k)
	}

	cfg := Parse(nil)
	if cfg.RunAddr != ":8080" {
		t.Fatalf("RunAddr=%q", cfg.RunAddr)
	}
	if cfg.BaseURL != "http://localhost:8080" {
		t.Fatalf("BaseURL=%q", cfg.BaseURL)
	}
	if cfg.FileStoragePath != "/tmp/short-url-db.json" {
		t.Fatalf("FileStoragePath=%q", cfg.FileStoragePath)
	}
}

func TestNewConfigUsesArgs(t *testing.T) {
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	for _, k := range []string{"SERVER_ADDRESS", "BASE_URL", "FILE_STORAGE_PATH", "DATABASE_DSN"} {
		_ = os.Unsetenv(k)
	}
	os.Args = []string{"shortener", "-a", ":9090"}
	cfg := NewConfig()
	if cfg.RunAddr != ":9090" {
		t.Fatalf("RunAddr=%q", cfg.RunAddr)
	}
}

func TestParseFlagsAndEnv(t *testing.T) {
	t.Setenv("SERVER_ADDRESS", "localhost:9090")
	t.Setenv("BASE_URL", "http://example.com/")
	t.Setenv("FILE_STORAGE_PATH", "/tmp/custom.json")
	t.Setenv("DATABASE_DSN", "postgres://x")

	cfg := Parse([]string{"-a", ":7070", "-b", "http://flag/", "-file-storage-path", "/tmp/flag.json", "-d", "postgres://flag"})
	if cfg.RunAddr != "localhost:9090" {
		t.Fatalf("env should override flag RunAddr, got %q", cfg.RunAddr)
	}
	if cfg.BaseURL != "http://example.com" {
		t.Fatalf("BaseURL=%q", cfg.BaseURL)
	}
	if cfg.FileStoragePath != "/tmp/custom.json" {
		t.Fatalf("FileStoragePath=%q", cfg.FileStoragePath)
	}
	if cfg.DatabaseDSN != "postgres://x" {
		t.Fatalf("DatabaseDSN=%q", cfg.DatabaseDSN)
	}
}
