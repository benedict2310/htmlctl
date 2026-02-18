package caddy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Site struct {
	Domain string
	Root   string
}

type ConfigOptions struct {
	DisableAutoHTTPS bool
}

func GenerateConfig(sites []Site) (string, error) {
	return GenerateConfigWithOptions(sites, ConfigOptions{})
}

func GenerateConfigWithOptions(sites []Site, opts ConfigOptions) (string, error) {
	ordered := append([]Site(nil), sites...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Domain < ordered[j].Domain
	})

	var b strings.Builder
	b.WriteString("# managed by htmlservd\n")
	if opts.DisableAutoHTTPS {
		b.WriteString("{\n")
		b.WriteString("\tauto_https off\n")
		b.WriteString("}\n")
	}
	if len(ordered) == 0 {
		return b.String(), nil
	}
	b.WriteString("\n")
	for _, site := range ordered {
		domain := strings.TrimSpace(site.Domain)
		root := strings.TrimSpace(site.Root)
		if domain == "" {
			return "", fmt.Errorf("site domain is required")
		}
		if root == "" {
			return "", fmt.Errorf("site root is required for domain %q", domain)
		}
		siteAddress := domain
		if opts.DisableAutoHTTPS {
			siteAddress = "http://" + domain
		}
		fmt.Fprintf(&b, "%s {\n", siteAddress)
		fmt.Fprintf(&b, "\troot * %s\n", root)
		b.WriteString("\tfile_server\n")
		b.WriteString("}\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func WriteConfig(path string, content string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("config path is required")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".caddy-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(0o644); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("chmod temporary config file: %w", err)
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temporary config file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temporary config file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace config file %s: %w", path, err)
	}

	cleanup = false
	return nil
}
