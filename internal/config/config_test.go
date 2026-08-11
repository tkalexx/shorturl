package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONConfigPriority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
  "server_address": "localhost:9090",
  "base_url": "http://from-file",
  "file_storage_path": "/tmp/from-file.json",
  "database_dsn": "postgres://file",
  "enable_https": true,
  "auth_secret": "file-auth-secret-16",
  "audit_file": "/tmp/audit-file.log",
  "pprof_addr": "localhost:7070"
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	resetFlags(t)
	os.Args = []string{"shortener", "-config", path, "-a", "localhost:1111"}
	t.Setenv("BASE_URL", "http://from-env")
	t.Setenv("AUTH_SECRET", "env-auth-secret-16chars")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}

	if cfg.RunAddr != "localhost:1111" {
		t.Fatalf("RunAddr=%q, want flag value", cfg.RunAddr)
	}
	if cfg.BaseURL != "http://from-env" {
		t.Fatalf("BaseURL=%q, want env value", cfg.BaseURL)
	}
	if cfg.FileStoragePath != "/tmp/from-file.json" {
		t.Fatalf("FileStoragePath=%q, want file value", cfg.FileStoragePath)
	}
	if cfg.DatabaseDSN != "postgres://file" {
		t.Fatalf("DatabaseDSN=%q, want file value", cfg.DatabaseDSN)
	}
	if !cfg.EnableHTTPS {
		t.Fatal("EnableHTTPS=false, want true from file")
	}
	if cfg.AuthSecret != "env-auth-secret-16chars" {
		t.Fatalf("AuthSecret=%q, want env value", cfg.AuthSecret)
	}
	if cfg.AuditFile != "/tmp/audit-file.log" {
		t.Fatalf("AuditFile=%q, want file value", cfg.AuditFile)
	}
	if cfg.PprofAddr != "localhost:7070" {
		t.Fatalf("PprofAddr=%q, want file value", cfg.PprofAddr)
	}
}

func TestCONFIGEnvPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(path, []byte(`{"server_address":"localhost:5555","auth_secret":"config-env-secret1"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	resetFlags(t)
	os.Args = []string{"shortener"}
	t.Setenv("CONFIG", path)

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.RunAddr != "localhost:5555" {
		t.Fatalf("RunAddr=%q, want from CONFIG file", cfg.RunAddr)
	}
}

func resetFlags(t *testing.T) {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	t.Cleanup(func() {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	})
}
