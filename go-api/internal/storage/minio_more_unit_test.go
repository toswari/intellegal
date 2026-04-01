//go:build !integration

package storage

import "testing"

func TestNewAdapter_PropagatesValidationErrors(t *testing.T) {
	_, err := NewAdapter(FactoryConfig{
		MinIOAccessKey: "minioadmin",
		MinIOSecretKey: "minioadmin",
		MinIOBucket:    "contracts",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Error() != "minio endpoint is empty" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestMinIOAdapterHealthCheck_ReturnsErrorWhenUninitialized(t *testing.T) {
	var adapter *MinIOAdapter

	err := adapter.HealthCheck(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "minio is not initialized" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}
