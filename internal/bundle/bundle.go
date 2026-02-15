package bundle

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

type Bundle struct {
	Manifest   Manifest
	Files      map[string][]byte
	ExtraFiles []string
}

type ValidationError struct {
	MissingFiles   []string
	HashMismatches []string
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, 2)
	if len(e.MissingFiles) > 0 {
		parts = append(parts, fmt.Sprintf("missing files: %s", strings.Join(e.MissingFiles, ", ")))
	}
	if len(e.HashMismatches) > 0 {
		parts = append(parts, fmt.Sprintf("hash mismatches: %s", strings.Join(e.HashMismatches, ", ")))
	}
	if len(parts) == 0 {
		return "invalid bundle"
	}
	return strings.Join(parts, "; ")
}

func ReadTar(r io.Reader) (Bundle, error) {
	b := Bundle{Files: map[string][]byte{}}
	tr := tar.NewReader(r)
	var manifestBytes []byte

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return b, fmt.Errorf("read tar entry: %w", err)
		}
		if hdr == nil || hdr.Typeflag == tar.TypeDir {
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			return b, fmt.Errorf("unsupported tar entry type for %q", hdr.Name)
		}
		name, err := sanitizeEntryPath(hdr.Name)
		if err != nil {
			return b, fmt.Errorf("invalid tar path %q: %w", hdr.Name, err)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return b, fmt.Errorf("read tar entry %q: %w", name, err)
		}
		if name == "manifest.json" {
			manifestBytes = content
			continue
		}
		if _, exists := b.Files[name]; exists {
			return b, fmt.Errorf("duplicate tar entry %q", name)
		}
		b.Files[name] = content
	}

	if len(manifestBytes) == 0 {
		return b, fmt.Errorf("bundle is missing manifest.json")
	}

	manifest, err := ParseManifest(manifestBytes)
	if err != nil {
		return b, err
	}
	b.Manifest = manifest

	expected := map[string]string{}
	for _, res := range manifest.Resources {
		if res.Deleted {
			continue
		}
		for _, ref := range res.FileEntries() {
			hash, err := CanonicalHash(ref.Hash)
			if err != nil {
				return b, fmt.Errorf("manifest resource %q file %q: %w", res.Name, ref.File, err)
			}
			if prev, ok := expected[ref.File]; ok && prev != hash {
				return b, fmt.Errorf("manifest defines conflicting hashes for %q", ref.File)
			}
			expected[ref.File] = hash
		}
	}

	validation := &ValidationError{}
	for file, expectedHash := range expected {
		content, ok := b.Files[file]
		if !ok {
			validation.MissingFiles = append(validation.MissingFiles, file)
			continue
		}
		sum := sha256.Sum256(content)
		actual := "sha256:" + hex.EncodeToString(sum[:])
		if actual != expectedHash {
			validation.HashMismatches = append(validation.HashMismatches, file)
		}
	}
	if len(validation.MissingFiles) > 0 || len(validation.HashMismatches) > 0 {
		sort.Strings(validation.MissingFiles)
		sort.Strings(validation.HashMismatches)
		return b, validation
	}

	for file := range b.Files {
		if _, ok := expected[file]; !ok {
			b.ExtraFiles = append(b.ExtraFiles, file)
		}
	}
	sort.Strings(b.ExtraFiles)

	return b, nil
}

func sanitizeEntryPath(name string) (string, error) {
	clean := path.Clean(strings.TrimSpace(strings.ReplaceAll(name, "\\", "/")))
	if err := validateBundlePath(clean); err != nil {
		return "", err
	}
	return clean, nil
}
