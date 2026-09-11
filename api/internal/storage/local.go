// LocalStore: blobs under one root dir, one file per key. Docker mounts a
// named volume there so files survive restarts (docker-compose.yml).
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStore struct {
	dir string
}

func NewLocal(dir string) *LocalStore {
	return &LocalStore{dir: dir}
}

func (s *LocalStore) Kind() string { return "local" }

// path cleans the key against the root — keys are server-generated, but the
// join must never escape dir (defense in depth for traversal input).
func (s *LocalStore) path(key string) string {
	return filepath.Join(s.dir, filepath.Clean("/"+key))
}

func (s *LocalStore) Put(_ context.Context, key, _ string, _ int64, r io.Reader) error {
	p := s.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err)
	}
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create %s: %w", p, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}

func (s *LocalStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	f, err := os.Open(s.path(key))
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	return f, nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	if err := os.Remove(s.path(key)); err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}
