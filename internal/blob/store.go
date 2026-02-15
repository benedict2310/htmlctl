package blob

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var hashHexPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) Path(hashHex string) string {
	return filepath.Join(s.root, hashHex)
}

func (s *Store) Put(ctx context.Context, hashHex string, content []byte) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !hashHexPattern.MatchString(hashHex) {
		return false, fmt.Errorf("invalid hash %q", hashHex)
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return false, fmt.Errorf("create blob directory %s: %w", s.root, err)
	}

	dst := s.Path(hashHex)
	if _, err := os.Stat(dst); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat blob %s: %w", dst, err)
	}

	tmp, err := os.CreateTemp(s.root, hashHex+".tmp-*")
	if err != nil {
		return false, fmt.Errorf("create temp blob file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return false, fmt.Errorf("write temp blob file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return false, fmt.Errorf("close temp blob file: %w", err)
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		if _, statErr := os.Stat(dst); statErr == nil {
			_ = os.Remove(tmpPath)
			return false, nil
		}
		_ = os.Remove(tmpPath)
		return false, fmt.Errorf("finalize blob %s: %w", dst, err)
	}
	return true, nil
}
