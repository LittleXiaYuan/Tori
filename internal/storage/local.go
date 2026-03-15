package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalBackend implements Backend using local filesystem.
type LocalBackend struct {
	basePath string
}

// NewLocalBackend creates a local filesystem storage backend.
func NewLocalBackend(basePath string) (*LocalBackend, error) {
	if basePath == "" {
		basePath = "data/storage"
	}
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &LocalBackend{basePath: basePath}, nil
}

func (b *LocalBackend) Write(_ context.Context, key string, data io.Reader) error {
	path := filepath.Join(b.basePath, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, data)
	return err
}

func (b *LocalBackend) Read(_ context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(b.basePath, filepath.FromSlash(key))
	return os.Open(path)
}

func (b *LocalBackend) List(_ context.Context, prefix string) ([]Object, error) {
	dir := filepath.Join(b.basePath, filepath.FromSlash(prefix))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var objects []Object
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		objects = append(objects, Object{
			Key:          filepath.ToSlash(filepath.Join(prefix, e.Name())),
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
	}
	return objects, nil
}

func (b *LocalBackend) Delete(_ context.Context, key string) error {
	path := filepath.Join(b.basePath, filepath.FromSlash(key))
	return os.Remove(path)
}

func (b *LocalBackend) Exists(_ context.Context, key string) (bool, error) {
	path := filepath.Join(b.basePath, filepath.FromSlash(key))
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (b *LocalBackend) Close() error { return nil }

// ── Cloud backends (stubs — return error until implemented) ──

// NewS3Backend creates an S3-compatible storage backend (not yet implemented).
func NewS3Backend(_ Config) (Backend, error) {
	return nil, fmt.Errorf("S3 backend not yet implemented; set STORAGE_BACKEND=local or leave empty")
}

// NewOSSBackend creates an Alibaba Cloud OSS backend (not yet implemented).
func NewOSSBackend(_ Config) (Backend, error) {
	return nil, fmt.Errorf("OSS backend not yet implemented; set STORAGE_BACKEND=local or leave empty")
}

// NewCOSBackend creates a Tencent Cloud COS backend (not yet implemented).
func NewCOSBackend(_ Config) (Backend, error) {
	return nil, fmt.Errorf("COS backend not yet implemented; set STORAGE_BACKEND=local or leave empty")
}

// NewOBSBackend creates a Huawei Cloud OBS backend (not yet implemented).
func NewOBSBackend(_ Config) (Backend, error) {
	return nil, fmt.Errorf("OBS backend not yet implemented; set STORAGE_BACKEND=local or leave empty")
}

// Compile-time check
var _ Backend = (*LocalBackend)(nil)

// ensure time is used
var _ = time.Now
