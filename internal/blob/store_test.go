package blob

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStorePutWritesAndDedupes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "blobs", "sha256")
	s := NewStore(root)
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	content := []byte("hello")

	created, err := s.Put(context.Background(), hash, content)
	if err != nil {
		t.Fatalf("Put() first error = %v", err)
	}
	if !created {
		t.Fatalf("expected created=true on first put")
	}
	got, err := os.ReadFile(s.Path(hash))
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("blob content mismatch")
	}

	created, err = s.Put(context.Background(), hash, []byte("ignored"))
	if err != nil {
		t.Fatalf("Put() second error = %v", err)
	}
	if created {
		t.Fatalf("expected created=false for duplicate hash")
	}
	got, err = os.ReadFile(s.Path(hash))
	if err != nil {
		t.Fatalf("read blob after duplicate put: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("blob content should remain original")
	}
}

func TestStorePutInvalidHash(t *testing.T) {
	s := NewStore(t.TempDir())
	if _, err := s.Put(context.Background(), "bad", []byte("x")); err == nil {
		t.Fatalf("expected invalid hash error")
	}
}
