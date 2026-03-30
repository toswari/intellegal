package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaultsWithToken(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("STORAGE_PROVIDER", "")
	t.Setenv("LOCAL_STORAGE_PATH", "")
	t.Setenv("DATABASE_PING_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.StorageProvider != "local" {
		t.Fatalf("expected default storage provider local, got %q", cfg.StorageProvider)
	}
	if cfg.LocalStoragePath != "./samples/storage" {
		t.Fatalf("expected default local storage path, got %q", cfg.LocalStoragePath)
	}
	if cfg.DatabasePingTimeout != 5*time.Second {
		t.Fatalf("expected default database ping timeout 5s, got %s", cfg.DatabasePingTimeout)
	}
}

func TestLoadFailsWithoutToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://app:app@localhost:5432/app")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when INTERNAL_SERVICE_TOKEN is missing")
	}
}

func TestLoadParsesLogLevel(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.LogLevel.String() != "DEBUG" {
		t.Fatalf("expected DEBUG log level, got %s", cfg.LogLevel.String())
	}
}

func TestLoadRejectsInvalidStorageProvider(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("STORAGE_PROVIDER", "s3")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid storage provider")
	}
	if !strings.Contains(err.Error(), "invalid STORAGE_PROVIDER") {
		t.Fatalf("expected storage provider validation error, got %v", err)
	}
}

func TestLoadRejectsInvalidDatabasePingTimeout(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DATABASE_PING_TIMEOUT", "zero")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid database ping timeout")
	}
}

func TestLoadRejectsInvalidExternalCopyRetries(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("EXTERNAL_COPY_API_RETRIES", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid external copy retries")
	}
}

func TestLoadRequiresAzureFields(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("STORAGE_PROVIDER", "azure")
	t.Setenv("AZURE_STORAGE_ACCOUNT", "")
	t.Setenv("AZURE_BLOB_CONTAINER", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing azure storage fields")
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgresql://app:app@localhost:5432/app")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "test-token")
}
