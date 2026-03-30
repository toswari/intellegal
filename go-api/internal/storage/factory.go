package storage

import "fmt"

type FactoryConfig struct {
	Provider           string
	LocalPath          string
	AzureAccountName   string
	AzureBlobContainer string
}

func NewAdapter(cfg FactoryConfig) (Adapter, error) {
	switch cfg.Provider {
	case "local":
		return NewLocalAdapter(cfg.LocalPath)
	case "azure":
		return NewAzureBlobAdapter(cfg.AzureAccountName, cfg.AzureBlobContainer)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %q", cfg.Provider)
	}
}
