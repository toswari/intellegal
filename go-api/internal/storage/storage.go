package storage

import (
	"context"
	"errors"
	"io"
)

var ErrNotImplemented = errors.New("storage adapter operation is not implemented")

// Adapter abstracts object storage used for uploaded contract files.
type Adapter interface {
	Put(ctx context.Context, key string, body io.Reader) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}
