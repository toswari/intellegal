package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultInternalToken = "change-me"

// Config holds runtime configuration for the go-api service.
type Config struct {
	Port                 int
	LogLevel             slog.Level
	ShutdownGracePeriod  time.Duration
	CORSAllowedOrigins   []string
	DatabaseURL          string
	DatabasePingTimeout  time.Duration
	InternalServiceToken string
	InternalAIBaseURL    string
	ExternalCopyBaseURL  string
	ExternalCopyToken    string
	ExternalCopyTimeout  time.Duration
	ExternalCopyRetries  int
	StorageProvider      string
	LocalStoragePath     string
	AzureStorageAccount  string
	AzureBlobContainer   string
}

func Load() (Config, error) {
	cfg := Config{
		Port:                 8080,
		LogLevel:             slog.LevelInfo,
		ShutdownGracePeriod:  10 * time.Second,
		CORSAllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		DatabasePingTimeout:  5 * time.Second,
		InternalServiceToken: defaultInternalToken,
		InternalAIBaseURL:    "http://localhost:8000",
		ExternalCopyTimeout:  8 * time.Second,
		ExternalCopyRetries:  3,
		StorageProvider:      "local",
		LocalStoragePath:     "./samples/storage",
	}

	if v := os.Getenv("GO_API_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port <= 0 || port > 65535 {
			return Config{}, fmt.Errorf("invalid GO_API_PORT: %q", v)
		}
		cfg.Port = port
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		level := strings.ToLower(strings.TrimSpace(v))
		switch level {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn", "warning":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			return Config{}, fmt.Errorf("invalid LOG_LEVEL: %q", v)
		}
	}

	if v := os.Getenv("SHUTDOWN_GRACE_PERIOD"); v != "" {
		dur, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SHUTDOWN_GRACE_PERIOD: %q", v)
		}
		cfg.ShutdownGracePeriod = dur
	}

	if v := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); v != "" {
		parts := strings.Split(v, ",")
		origins := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		if len(origins) == 0 {
			return Config{}, errors.New("CORS_ALLOWED_ORIGINS is set but empty")
		}
		cfg.CORSAllowedOrigins = origins
	}

	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		cfg.DatabaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("DATABASE_PING_TIMEOUT")); v != "" {
		dur, err := time.ParseDuration(v)
		if err != nil || dur <= 0 {
			return Config{}, fmt.Errorf("invalid DATABASE_PING_TIMEOUT: %q", v)
		}
		cfg.DatabasePingTimeout = dur
	}

	if v := os.Getenv("INTERNAL_SERVICE_TOKEN"); v != "" {
		cfg.InternalServiceToken = v
	}

	if v := strings.TrimSpace(os.Getenv("INTERNAL_AI_BASE_URL")); v != "" {
		cfg.InternalAIBaseURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("EXTERNAL_COPY_API_BASE_URL")); v != "" {
		cfg.ExternalCopyBaseURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("EXTERNAL_COPY_API_TOKEN")); v != "" {
		cfg.ExternalCopyToken = v
	}
	if v := strings.TrimSpace(os.Getenv("EXTERNAL_COPY_API_TIMEOUT")); v != "" {
		dur, err := time.ParseDuration(v)
		if err != nil || dur <= 0 {
			return Config{}, fmt.Errorf("invalid EXTERNAL_COPY_API_TIMEOUT: %q", v)
		}
		cfg.ExternalCopyTimeout = dur
	}
	if v := strings.TrimSpace(os.Getenv("EXTERNAL_COPY_API_RETRIES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 10 {
			return Config{}, fmt.Errorf("invalid EXTERNAL_COPY_API_RETRIES: %q", v)
		}
		cfg.ExternalCopyRetries = n
	}

	if v := strings.TrimSpace(os.Getenv("STORAGE_PROVIDER")); v != "" {
		switch strings.ToLower(v) {
		case "local", "azure":
			cfg.StorageProvider = strings.ToLower(v)
		default:
			return Config{}, fmt.Errorf("invalid STORAGE_PROVIDER: %q", v)
		}
	}

	if v := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_PATH")); v != "" {
		cfg.LocalStoragePath = v
	}

	if v := strings.TrimSpace(os.Getenv("AZURE_STORAGE_ACCOUNT")); v != "" {
		cfg.AzureStorageAccount = v
	}

	if v := strings.TrimSpace(os.Getenv("AZURE_BLOB_CONTAINER")); v != "" {
		cfg.AzureBlobContainer = v
	}

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, errors.New("DATABASE_URL is not set")
	}

	if cfg.InternalServiceToken == defaultInternalToken {
		return Config{}, errors.New("INTERNAL_SERVICE_TOKEN is not set")
	}

	if cfg.StorageProvider == "local" && strings.TrimSpace(cfg.LocalStoragePath) == "" {
		return Config{}, errors.New("LOCAL_STORAGE_PATH is not set")
	}

	if cfg.StorageProvider == "azure" {
		if strings.TrimSpace(cfg.AzureStorageAccount) == "" {
			return Config{}, errors.New("AZURE_STORAGE_ACCOUNT is not set")
		}
		if strings.TrimSpace(cfg.AzureBlobContainer) == "" {
			return Config{}, errors.New("AZURE_BLOB_CONTAINER is not set")
		}
	}

	return cfg, nil
}
