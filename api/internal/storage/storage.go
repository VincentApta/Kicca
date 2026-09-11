// Package storage abstracts task-attachment blobs: S3 when configured, a
// local directory otherwise (dev / single-node deploys). One bucket for the
// whole app, never per-project (docs/domain.md rule 10).
package storage

import (
	"context"
	"io"
	"os"
)

// Store is the blob backend attachment handlers talk to. Keys are
// server-generated (uuid + extension); implementations must not trust them.
type Store interface {
	Put(ctx context.Context, key, contentType string, size int64, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Kind() string // "local" | "s3" — persisted on each attachment row
}

// NewFromEnv picks S3 iff S3_BUCKET is set (credentials via the default AWS
// chain, optional S3_ENDPOINT for minio later); otherwise LocalStore rooted
// at ATTACHMENTS_DIR (default /data/attachments).
func NewFromEnv() Store {
	if bucket := os.Getenv("S3_BUCKET"); bucket != "" {
		return newS3(bucket, os.Getenv("S3_REGION"), os.Getenv("S3_ENDPOINT"))
	}
	return NewLocal(envOr("ATTACHMENTS_DIR", "/data/attachments"))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
