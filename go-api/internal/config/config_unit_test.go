//go:build !integration

package config

import (
	"testing"
	"time"
)

func TestLoad_ReturnsDefaultsWhenRequiredTokenIsPresent(t *testing.T) {
	// Arrange
	setRequiredEnv(t)
	t.Setenv("MINIO_ENDPOINT", "")
	t.Setenv("MINIO_BUCKET", "")
	t.Setenv("DATABASE_PING_TIMEOUT", "")

	// Act
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Assert
	if cfg.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.MinIOEndpoint != "minio:9000" {
		t.Fatalf("expected default minio endpoint, got %q", cfg.MinIOEndpoint)
	}
	if cfg.MinIOBucket != "contracts" {
		t.Fatalf("expected default minio bucket, got %q", cfg.MinIOBucket)
	}
	if cfg.DatabasePingTimeout != 5*time.Second {
		t.Fatalf("expected default database ping timeout 5s, got %s", cfg.DatabasePingTimeout)
	}
	if cfg.InternalAITimeout != 90*time.Second {
		t.Fatalf("expected default internal ai timeout 90s, got %s", cfg.InternalAITimeout)
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		t.Fatal("expected default cors allowed origins")
	}
	if cfg.CORSAllowedOrigins[0] != "http://localhost:3000" {
		t.Fatalf("expected localhost:3000 as first default cors origin, got %q", cfg.CORSAllowedOrigins[0])
	}
}

func TestLoad_ReturnsErrorWhenInternalServiceTokenIsMissing(t *testing.T) {
	// Arrange
	t.Setenv("DATABASE_URL", "postgresql://app:app@localhost:5432/app")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error when INTERNAL_SERVICE_TOKEN is missing")
	}
}

func TestLoad_ParsesLogLevel(t *testing.T) {
	// Arrange
	setRequiredEnv(t)
	t.Setenv("LOG_LEVEL", "debug")

	// Act
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Assert
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected debug log level, got %s", cfg.LogLevel)
	}
}

func TestLoad_ReturnsErrorForInvalidDatabasePingTimeout(t *testing.T) {
	// Arrange
	setRequiredEnv(t)
	t.Setenv("DATABASE_PING_TIMEOUT", "zero")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error for invalid database ping timeout")
	}
}

func TestLoad_ReturnsErrorForInvalidInternalAITimeout(t *testing.T) {
	// Arrange
	setRequiredEnv(t)
	t.Setenv("INTERNAL_AI_TIMEOUT", "zero")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error for invalid internal ai timeout")
	}
}

func TestLoad_ReturnsErrorForInvalidExternalCopyRetries(t *testing.T) {
	// Arrange
	setRequiredEnv(t)
	t.Setenv("EXTERNAL_COPY_API_RETRIES", "0")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error for invalid external copy retries")
	}
}

func TestLoad_ReturnsErrorWhenRequiredMinIOFieldsAreMissing(t *testing.T) {
	// Arrange
	setRequiredEnv(t)
	t.Setenv("MINIO_ACCESS_KEY", "")
	t.Setenv("MINIO_SECRET_KEY", "")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error for missing minio storage fields")
	}
}

func TestLoad_ParsesCORSAllowedOrigins(t *testing.T) {
	// Arrange
	setRequiredEnv(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://app.example.com")

	// Act
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Assert
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("expected 2 cors origins, got %d", len(cfg.CORSAllowedOrigins))
	}
	if cfg.CORSAllowedOrigins[1] != "https://app.example.com" {
		t.Fatalf("unexpected second cors origin: %q", cfg.CORSAllowedOrigins[1])
	}
}

func TestLoad_ReturnsErrorWhenCORSAllowedOriginsAreEmpty(t *testing.T) {
	// Arrange
	setRequiredEnv(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", " , ")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error for empty CORS_ALLOWED_ORIGINS")
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgresql://app:app@localhost:5432/app")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "test-token")
	t.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	t.Setenv("MINIO_SECRET_KEY", "minioadmin")
}
