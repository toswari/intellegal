package storage

import "fmt"

type FactoryConfig struct {
	Provider           string
	LocalPath          string
	AzureAccountName   string
	AzureBlobContainer string
	MinIOEndpoint      string
	MinIOAccessKey     string
	MinIOSecretKey     string
	MinIOBucket        string
	MinIOUseSSL        bool
}

func NewAdapter(cfg FactoryConfig) (Adapter, error) {
	switch cfg.Provider {
	case "local":
		return NewLocalAdapter(cfg.LocalPath)
	case "azure":
		return NewAzureBlobAdapter(cfg.AzureAccountName, cfg.AzureBlobContainer)
	case "minio":
		return NewMinIOAdapter(MinIOConfig{
			Endpoint:  cfg.MinIOEndpoint,
			AccessKey: cfg.MinIOAccessKey,
			SecretKey: cfg.MinIOSecretKey,
			Bucket:    cfg.MinIOBucket,
			UseSSL:    cfg.MinIOUseSSL,
		})
	default:
		return nil, fmt.Errorf("unsupported storage provider: %q", cfg.Provider)
	}
}
