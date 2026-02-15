package bundle

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	ApplyModeFull    = "full"
	ApplyModePartial = "partial"
)

var sha256HexPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Manifest struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Mode       string     `json:"mode"`
	Website    string     `json:"website"`
	Resources  []Resource `json:"resources"`
}

type Resource struct {
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	File        string    `json:"file,omitempty"`
	Hash        string    `json:"hash,omitempty"`
	Files       []FileRef `json:"files,omitempty"`
	ContentType string    `json:"contentType,omitempty"`
	Size        int64     `json:"size,omitempty"`
	Deleted     bool      `json:"deleted,omitempty"`
}

type FileRef struct {
	File string `json:"file"`
	Hash string `json:"hash"`
}

func ParseManifest(b []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("parse manifest json: %w", err)
	}
	if err := m.Validate(); err != nil {
		return m, err
	}
	return m, nil
}

func (m Manifest) Validate() error {
	switch m.Mode {
	case ApplyModeFull, ApplyModePartial:
	default:
		return fmt.Errorf("manifest.mode must be %q or %q", ApplyModeFull, ApplyModePartial)
	}
	if strings.TrimSpace(m.Website) == "" {
		return fmt.Errorf("manifest.website is required")
	}
	if len(m.Resources) == 0 {
		return fmt.Errorf("manifest.resources must not be empty")
	}
	seen := make(map[string]struct{}, len(m.Resources))
	for i, r := range m.Resources {
		if err := r.validate(m.Mode); err != nil {
			return fmt.Errorf("manifest.resources[%d]: %w", i, err)
		}
		key := strings.ToLower(strings.TrimSpace(r.Kind)) + "\x00" + strings.TrimSpace(r.Name)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("manifest.resources[%d]: duplicate resource %s %q", i, r.Kind, r.Name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (r Resource) FileEntries() []FileRef {
	out := make([]FileRef, 0, len(r.Files)+1)
	if strings.TrimSpace(r.File) != "" || strings.TrimSpace(r.Hash) != "" {
		out = append(out, FileRef{File: strings.TrimSpace(r.File), Hash: strings.TrimSpace(r.Hash)})
	}
	for _, f := range r.Files {
		out = append(out, FileRef{File: strings.TrimSpace(f.File), Hash: strings.TrimSpace(f.Hash)})
	}
	return out
}

func (r Resource) validate(mode string) error {
	kind := strings.ToLower(strings.TrimSpace(r.Kind))
	switch kind {
	case "component", "page", "stylebundle", "asset", "script":
	default:
		return fmt.Errorf("unsupported resource kind %q", r.Kind)
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if r.Deleted {
		if mode != ApplyModePartial {
			return fmt.Errorf("deleted resources are only allowed in partial mode")
		}
		if kind == "asset" || kind == "script" {
			file := strings.TrimSpace(r.File)
			if file == "" {
				return fmt.Errorf("deleted %s resources require file", kind)
			}
			if err := validateBundlePath(file); err != nil {
				return fmt.Errorf("invalid file path %q: %w", file, err)
			}
			if strings.TrimSpace(r.Name) != file {
				return fmt.Errorf("%s name must match file path %q", kind, file)
			}
		}
		return nil
	}
	entries := r.FileEntries()
	if len(entries) == 0 {
		return fmt.Errorf("at least one file entry is required")
	}
	for _, e := range entries {
		if err := validateBundlePath(e.File); err != nil {
			return fmt.Errorf("invalid file path %q: %w", e.File, err)
		}
		if _, err := HashHex(e.Hash); err != nil {
			return fmt.Errorf("invalid hash for file %q: %w", e.File, err)
		}
	}
	switch kind {
	case "component", "page", "asset", "script":
		if len(entries) != 1 {
			return fmt.Errorf("%s resources must reference exactly one file", kind)
		}
	}
	if kind == "asset" || kind == "script" {
		if strings.TrimSpace(r.Name) != entries[0].File {
			return fmt.Errorf("%s name must match file path %q", kind, entries[0].File)
		}
	}
	return nil
}

func CanonicalHash(hash string) (string, error) {
	hex, err := HashHex(hash)
	if err != nil {
		return "", err
	}
	return "sha256:" + hex, nil
}

func HashHex(hash string) (string, error) {
	v := strings.TrimSpace(strings.ToLower(hash))
	if strings.HasPrefix(v, "sha256:") {
		v = strings.TrimPrefix(v, "sha256:")
	}
	if !sha256HexPattern.MatchString(v) {
		return "", fmt.Errorf("expected sha256 hex digest")
	}
	return v, nil
}

func validateBundlePath(p string) error {
	clean := path.Clean(strings.TrimSpace(strings.ReplaceAll(p, "\\", "/")))
	if clean == "." || clean == "" {
		return fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(clean, "/") {
		return fmt.Errorf("path must be relative")
	}
	if strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") || clean == ".." {
		return fmt.Errorf("path traversal is not allowed")
	}
	return nil
}
