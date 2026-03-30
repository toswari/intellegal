package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalAdapter struct {
	baseDir string
}

func NewLocalAdapter(baseDir string) (*LocalAdapter, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, errors.New("local storage path is empty")
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve local storage path: %w", err)
	}

	if err := os.MkdirAll(absBase, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage path: %w", err)
	}

	return &LocalAdapter{baseDir: absBase}, nil
}

func (a *LocalAdapter) Put(_ context.Context, key string, body io.Reader) (string, error) {
	target, err := a.resolvePath(key)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create parent directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	copied, copyErr := io.Copy(tmp, body)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write object (%d bytes): %w", copied, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("close temp file: %w", closeErr)
	}

	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("move temp file: %w", err)
	}

	return fmt.Sprintf("file://%s", filepath.ToSlash(target)), nil
}

func (a *LocalAdapter) Get(_ context.Context, key string) (io.ReadCloser, error) {
	target, err := a.resolvePath(key)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(target)
	if err != nil {
		return nil, fmt.Errorf("open object: %w", err)
	}
	return file, nil
}

func (a *LocalAdapter) resolvePath(key string) (string, error) {
	cleanKey := filepath.Clean(strings.TrimSpace(key))
	if cleanKey == "." || cleanKey == "" {
		return "", errors.New("storage key is empty")
	}
	if filepath.IsAbs(cleanKey) {
		return "", fmt.Errorf("absolute storage key is not allowed: %q", key)
	}

	target := filepath.Join(a.baseDir, cleanKey)
	rel, err := filepath.Rel(a.baseDir, target)
	if err != nil {
		return "", fmt.Errorf("validate storage key: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage key escapes base directory: %q", key)
	}

	return target, nil
}
