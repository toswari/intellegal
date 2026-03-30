package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNewAdapterLocal(t *testing.T) {
	adapter, err := NewAdapter(FactoryConfig{Provider: "local", LocalPath: t.TempDir()})
	if err != nil {
		t.Fatalf("expected local adapter, got error: %v", err)
	}
	if _, ok := adapter.(*LocalAdapter); !ok {
		t.Fatalf("expected LocalAdapter type, got %T", adapter)
	}
}

func TestNewAdapterAzurePlaceholder(t *testing.T) {
	adapter, err := NewAdapter(FactoryConfig{
		Provider:           "azure",
		AzureAccountName:   "example",
		AzureBlobContainer: "contracts",
	})
	if err != nil {
		t.Fatalf("expected azure adapter, got error: %v", err)
	}

	_, err = adapter.Put(context.Background(), "doc.pdf", strings.NewReader("content"))
	if err == nil {
		t.Fatal("expected placeholder put to fail")
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

func TestNewAdapterUnsupportedProvider(t *testing.T) {
	_, err := NewAdapter(FactoryConfig{Provider: "s3"})
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
}
