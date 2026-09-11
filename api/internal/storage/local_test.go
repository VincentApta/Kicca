package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestLocalStoreRoundTrip(t *testing.T) {
	s := NewLocal(t.TempDir())
	ctx := context.Background()
	if s.Kind() != "local" {
		t.Fatalf("kind: %s", s.Kind())
	}
	body := []byte("attachment-bytes")
	if err := s.Put(ctx, "a/b/c.png", "image/png", int64(len(body)), bytes.NewReader(body)); err != nil {
		t.Fatalf("put: %v", err)
	}
	rc, err := s.Open(ctx, "a/b/c.png")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, body) {
		t.Fatalf("round-trip: %q", got)
	}
	if err := s.Delete(ctx, "a/b/c.png"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Open(ctx, "a/b/c.png"); err == nil {
		t.Fatal("open after delete: want error")
	}
}

// traversal input must never escape the root dir.
func TestLocalStoreKeyTraversal(t *testing.T) {
	s := NewLocal(t.TempDir())
	if p := s.path("../../etc/passwd"); p == "/etc/passwd" || !bytes.HasPrefix([]byte(p), []byte(s.dir)) {
		t.Fatalf("traversal escaped root: %s", p)
	}
}
