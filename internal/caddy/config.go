package caddy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	backendpkg "github.com/benedict2310/htmlctl/internal/backend"
)

type Backend struct {
	PathPrefix string
	Upstream   string
}

type Site struct {
	Domain   string
	Root     string
	Backends []Backend
}

type ConfigOptions struct {
	DisableAutoHTTPS bool
	TelemetryPort    int
}

func GenerateConfig(sites []Site) (string, error) {
	return GenerateConfigWithOptions(sites, ConfigOptions{})
}

func GenerateConfigWithOptions(sites []Site, opts ConfigOptions) (string, error) {
	if opts.TelemetryPort < 0 || opts.TelemetryPort > 65535 {
		return "", fmt.Errorf("telemetry port must be in range 0..65535")
	}

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
		if strings.ContainsAny(root, "\n{}") {
			return "", fmt.Errorf("site root for domain %q contains forbidden characters", domain)
		}
		siteAddress := domain
		if opts.DisableAutoHTTPS {
			siteAddress = "http://" + domain
		}
		backends := append([]Backend(nil), site.Backends...)
		sort.Slice(backends, func(i, j int) bool {
			return backends[i].PathPrefix < backends[j].PathPrefix
		})
		for _, backend := range backends {
			if _, err := backendpkg.ValidatePathPrefix(backend.PathPrefix); err != nil {
				return "", fmt.Errorf("invalid backend path prefix for domain %q: %w", domain, err)
			}
			if _, err := backendpkg.ValidateUpstreamURL(backend.Upstream); err != nil {
				return "", fmt.Errorf("invalid backend upstream for domain %q: %w", domain, err)
			}
		}

		fmt.Fprintf(&b, "%s {\n", siteAddress)
		fmt.Fprintf(&b, "\troot * %s\n", root)
		if opts.TelemetryPort > 0 {
			b.WriteString("\thandle /collect/v1/events* {\n")
			fmt.Fprintf(&b, "\t\treverse_proxy 127.0.0.1:%d\n", opts.TelemetryPort)
			b.WriteString("\t}\n")
		}
		for _, backend := range backends {
			fmt.Fprintf(&b, "\treverse_proxy %s %s\n", backend.PathPrefix, backend.Upstream)
		}
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
