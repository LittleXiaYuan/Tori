package storage

import (
	"context"
	"io"
	"time"
)

// Backend defines the interface for log storage backends (local, S3, OSS, etc.)
type Backend interface {
	// Write writes data to the storage backend
	Write(ctx context.Context, key string, data io.Reader) error

	// Read reads data from the storage backend
	Read(ctx context.Context, key string) (io.ReadCloser, error)

	// List lists objects with the given prefix
	List(ctx context.Context, prefix string) ([]Object, error)

	// Delete deletes an object
	Delete(ctx context.Context, key string) error

	// Exists checks if an object exists
	Exists(ctx context.Context, key string) (bool, error)

	// Close closes the backend connection
	Close() error
}

// Object represents a stored object
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
}

// Config is the configuration for storage backends
type Config struct {
	Backend   string // "local", "s3", "oss", "cos", "obs"
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Endpoint  string
	Prefix    string
	LocalPath string // for local backend
}

// NewBackend creates a new storage backend based on config
func NewBackend(cfg Config) (Backend, error) {
	switch cfg.Backend {
	case "local", "":
		return NewLocalBackend(cfg.LocalPath)
	case "s3":
		return NewS3Backend(cfg)
	case "oss":
		return NewOSSBackend(cfg)
	case "cos":
		return NewCOSBackend(cfg)
	case "obs":
		return NewOBSBackend(cfg)
	default:
		return NewLocalBackend(cfg.LocalPath)
	}
}

