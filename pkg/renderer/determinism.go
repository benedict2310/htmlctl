package renderer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const hashPrefixLength = 12

func normalizeLFString(input string) string {
	return strings.ReplaceAll(input, "\r\n", "\n")
}

func normalizeLFBytes(input []byte) []byte {
	return []byte(normalizeLFString(string(input)))
}

func hashHex(input []byte) string {
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}

func hashedFilename(filename string, content []byte) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	h := hashHex(content)
	if len(h) > hashPrefixLength {
		h = h[:hashPrefixLength]
	}
	if ext == "" {
		return fmt.Sprintf("%s-%s", name, h)
	}
	return fmt.Sprintf("%s-%s%s", name, h, ext)
}

func writeFileAtomic(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("write temp file %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp file %s to %s: %w", tmp, path, err)
	}
	return nil
}
