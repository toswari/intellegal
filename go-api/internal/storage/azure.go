package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type AzureBlobAdapter struct {
	accountName string
	container   string
}

func NewAzureBlobAdapter(accountName, container string) (*AzureBlobAdapter, error) {
	if strings.TrimSpace(accountName) == "" {
		return nil, fmt.Errorf("azure storage account name is empty")
	}
	if strings.TrimSpace(container) == "" {
		return nil, fmt.Errorf("azure blob container is empty")
	}
	return &AzureBlobAdapter{accountName: accountName, container: container}, nil
}

func (a *AzureBlobAdapter) Put(_ context.Context, _ string, _ io.Reader) (string, error) {
	return "", fmt.Errorf("azure blob put (%s/%s): %w", a.accountName, a.container, ErrNotImplemented)
}

func (a *AzureBlobAdapter) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("azure blob get (%s/%s): %w", a.accountName, a.container, ErrNotImplemented)
}
