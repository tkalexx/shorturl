package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tkalexx/shorturl.git/internal/auth"
)

// Config хранит настройки сервиса.
type Config struct {
	RunAddr         string // адрес и порт
	BaseURL         string // базовый адрес сокращенных ссылок
	FileStoragePath string // путь к файлу хранения URL
	DatabaseDSN     string // строка подключения к PostgreSQL
	AuthSecret      string // секрет подписи auth-cookie
	AuditFile       string // путь к файлу аудита; пусто - аудит в файл отключён
	AuditURL        string // URL удалённого приёмника аудита; пусто - отключён
	PprofAddr       string // адрес pprof-сервера; пусто - pprof отключён
	EnableHTTPS     bool   // включать HTTPS (-s / ENABLE_HTTPS)
}

// fileConfig описывает JSON-файл конфигурации.
type fileConfig struct {
	ServerAddress   string `json:"server_address"`
	BaseURL         string `json:"base_url"`
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`
	AuthSecret      string `json:"auth_secret"`
	AuditFile       string `json:"audit_file"`
	AuditURL        string `json:"audit_url"`
	PprofAddr       string `json:"pprof_addr"`
	EnableHTTPS     *bool  `json:"enable_https"`
}

// NewConfig инициализирует конфигурацию.
func NewConfig() (*Config, error) {
	var (
		runAddr         string
		baseURL         string
		fileStoragePath string
		databaseDSN     string
		authSecret      string
		auditFile       string
		auditURL        string
		pprofAddr       string
		enableHTTPS     bool
		configPath      string
	)

	flag.StringVar(&runAddr, "a", "", "address and port to run server (e.g., localhost:8888)")
	flag.StringVar(&baseURL, "b", "", "base URL for shortened links (e.g., http://localhost:8000/q)")
	flag.StringVar(&fileStoragePath, "f", "", "path to file storage for URLs")
	flag.StringVar(&fileStoragePath, "file-storage-path", "", "path to file storage for URLs")
	flag.StringVar(&databaseDSN, "d", "", "database connection string")
	flag.StringVar(&databaseDSN, "database-dsn", "", "database connection string")
	flag.StringVar(&authSecret, "auth-secret", "", "secret key for signing auth cookies")
	flag.StringVar(&auditFile, "audit-file", "", "path to audit log file")
	flag.StringVar(&auditURL, "audit-url", "", "remote audit receiver URL")
	flag.StringVar(&pprofAddr, "pprof", "", "address for pprof endpoints (empty to disable)")
	flag.BoolVar(&enableHTTPS, "s", false, "enable HTTPS")
	flag.StringVar(&configPath, "c", "", "path to JSON config file")
	flag.StringVar(&configPath, "config", "", "path to JSON config file")
	flag.Parse()

	if configPath == "" {
		configPath = os.Getenv("CONFIG")
	}

	cfg := &Config{
		RunAddr:         ":8080",
		FileStoragePath: "/tmp/short-url-db.json",
		PprofAddr:       "localhost:6060",
	}

	if configPath != "" {
		if err := applyFileConfig(cfg, configPath); err != nil {
			return nil, err
		}
	}

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "a":
			cfg.RunAddr = runAddr
		case "b":
			cfg.BaseURL = baseURL
		case "f", "file-storage-path":
			cfg.FileStoragePath = fileStoragePath
		case "d", "database-dsn":
			cfg.DatabaseDSN = databaseDSN
		case "auth-secret":
			cfg.AuthSecret = authSecret
		case "audit-file":
			cfg.AuditFile = auditFile
		case "audit-url":
			cfg.AuditURL = auditURL
		case "pprof":
			cfg.PprofAddr = pprofAddr
		case "s":
			cfg.EnableHTTPS = enableHTTPS
		}
	})

	applyEnv(cfg)
	finalize(cfg)
	return cfg, nil
}

func applyFileConfig(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}

	if fc.ServerAddress != "" {
		cfg.RunAddr = fc.ServerAddress
	}
	if fc.BaseURL != "" {
		cfg.BaseURL = fc.BaseURL
	}
	if fc.FileStoragePath != "" {
		cfg.FileStoragePath = fc.FileStoragePath
	}
	if fc.DatabaseDSN != "" {
		cfg.DatabaseDSN = fc.DatabaseDSN
	}
	if fc.AuthSecret != "" {
		cfg.AuthSecret = fc.AuthSecret
	}
	if fc.AuditFile != "" {
		cfg.AuditFile = fc.AuditFile
	}
	if fc.AuditURL != "" {
		cfg.AuditURL = fc.AuditURL
	}
	if fc.PprofAddr != "" {
		cfg.PprofAddr = fc.PprofAddr
	}
	if fc.EnableHTTPS != nil {
		cfg.EnableHTTPS = *fc.EnableHTTPS
	}
	return nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("SERVER_ADDRESS"); v != "" {
		cfg.RunAddr = v
	}
	if v := os.Getenv("BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("FILE_STORAGE_PATH"); v != "" {
		cfg.FileStoragePath = v
	}
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := os.Getenv("AUTH_SECRET"); v != "" {
		cfg.AuthSecret = v
	}
	if v := os.Getenv("AUDIT_FILE"); v != "" {
		cfg.AuditFile = v
	}
	if v := os.Getenv("AUDIT_URL"); v != "" {
		cfg.AuditURL = v
	}
	if v := os.Getenv("PPROF_ADDR"); v != "" {
		cfg.PprofAddr = v
	}
	if v := os.Getenv("ENABLE_HTTPS"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.EnableHTTPS = b
		} else {
			cfg.EnableHTTPS = true
		}
	}
}

func finalize(cfg *Config) {
	if cfg.FileStoragePath == "" {
		cfg.FileStoragePath = "/tmp/short-url-db.json"
	}

	if cfg.BaseURL == "" {
		addr := cfg.RunAddr
		if strings.HasPrefix(addr, ":") {
			addr = "localhost" + addr
		}
		scheme := "http"
		if cfg.EnableHTTPS {
			scheme = "https"
		}
		cfg.BaseURL = scheme + "://" + addr
	}

	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")
}

// Validate проверяет обязательные настройки сервиса.
func (c *Config) Validate() error {
	if _, err := auth.NewManager(c.AuthSecret); err != nil {
		return errors.New("invalid auth secret: " + err.Error())
	}
	return nil
}
