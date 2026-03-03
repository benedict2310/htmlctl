package caddy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	authpolicypkg "github.com/benedict2310/htmlctl/internal/authpolicy"
	backendpkg "github.com/benedict2310/htmlctl/internal/backend"
)

type Backend struct {
	PathPrefix string
	Upstream   string
}

type AuthPolicy struct {
	PathPrefix   string
	Username     string
	PasswordHash string
}

type Site struct {
	Domain        string
	Root          string
	Backends      []Backend
	AuthPolicies  []AuthPolicy
	Headers       map[string]string
	RespondStatus int
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
		iWildcard := strings.HasPrefix(strings.TrimSpace(ordered[i].Domain), "*.")
		jWildcard := strings.HasPrefix(strings.TrimSpace(ordered[j].Domain), "*.")
		if iWildcard != jWildcard {
			return !iWildcard
		}
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
		if site.RespondStatus == 0 && root == "" {
			return "", fmt.Errorf("site root is required for domain %q", domain)
		}
		if root != "" && strings.ContainsAny(root, "\n{}") {
			return "", fmt.Errorf("site root for domain %q contains forbidden characters", domain)
		}
		if site.RespondStatus < 0 || site.RespondStatus > 999 {
			return "", fmt.Errorf("site response status for domain %q must be in range 0..999", domain)
		}
		siteAddress := domain
		if opts.DisableAutoHTTPS {
			siteAddress = "http://" + domain
		}
		backends := append([]Backend(nil), site.Backends...)
		authPolicies := append([]AuthPolicy(nil), site.AuthPolicies...)
		headers := make([]string, 0, len(site.Headers))
		for key := range site.Headers {
			headers = append(headers, key)
		}
		sort.Strings(headers)
		sort.Slice(backends, func(i, j int) bool {
			return comparePathPrefixSpecificity(backends[i].PathPrefix, backends[j].PathPrefix)
		})
		sort.Slice(authPolicies, func(i, j int) bool {
			return authPolicies[i].PathPrefix < authPolicies[j].PathPrefix
		})
		for _, backend := range backends {
			if _, err := backendpkg.ValidatePathPrefix(backend.PathPrefix); err != nil {
				return "", fmt.Errorf("invalid backend path prefix for domain %q: %w", domain, err)
			}
			if _, err := backendpkg.ValidateUpstreamURL(backend.Upstream); err != nil {
				return "", fmt.Errorf("invalid backend upstream for domain %q: %w", domain, err)
			}
		}
		for _, policy := range authPolicies {
			if _, err := backendpkg.ValidatePathPrefix(policy.PathPrefix); err != nil {
				return "", fmt.Errorf("invalid auth policy path prefix for domain %q: %w", domain, err)
			}
			if _, err := authpolicypkg.ValidateUsername(policy.Username); err != nil {
				return "", fmt.Errorf("invalid auth policy username for domain %q: %w", domain, err)
			}
			if _, err := authpolicypkg.ValidatePasswordHash(policy.PasswordHash); err != nil {
				return "", fmt.Errorf("invalid auth policy password hash for domain %q: %w", domain, err)
			}
			if strings.ContainsAny(policy.PasswordHash, "\n\r{}") {
				return "", fmt.Errorf("auth policy password hash for domain %q contains forbidden characters", domain)
			}
			if opts.TelemetryPort > 0 && backendpkg.PathPrefixOverlapsPath(policy.PathPrefix, "/collect/v1/events") {
				return "", fmt.Errorf("auth policy prefix %q overlaps reserved telemetry endpoint for domain %q", policy.PathPrefix, domain)
			}
		}
		for i := 0; i < len(authPolicies); i++ {
			for j := i + 1; j < len(authPolicies); j++ {
				if backendpkg.PathPrefixesOverlap(authPolicies[i].PathPrefix, authPolicies[j].PathPrefix) {
					return "", fmt.Errorf("overlapping auth policy prefixes for domain %q: %s and %s", domain, authPolicies[i].PathPrefix, authPolicies[j].PathPrefix)
				}
			}
		}
		for _, policy := range authPolicies {
			for _, backend := range backends {
				if backendpkg.PathPrefixesOverlap(policy.PathPrefix, backend.PathPrefix) && policy.PathPrefix != backend.PathPrefix {
					return "", fmt.Errorf("auth policy prefix %q overlaps backend prefix %q for domain %q; only exact matches are allowed", policy.PathPrefix, backend.PathPrefix, domain)
				}
			}
		}

		fmt.Fprintf(&b, "%s {\n", siteAddress)
		for _, key := range headers {
			headerValue := strings.TrimSpace(site.Headers[key])
			if strings.TrimSpace(key) == "" {
				return "", fmt.Errorf("site header name is required for domain %q", domain)
			}
			if strings.ContainsAny(key, "\n\r{}") || strings.ContainsAny(headerValue, "\n\r{}") {
				return "", fmt.Errorf("site header for domain %q contains forbidden characters", domain)
			}
			fmt.Fprintf(&b, "\theader %s %q\n", key, headerValue)
		}
		if site.RespondStatus > 0 {
			fmt.Fprintf(&b, "\trespond %d\n", site.RespondStatus)
			b.WriteString("}\n\n")
			continue
		}
		fmt.Fprintf(&b, "\troot * %s\n", root)
		if opts.TelemetryPort > 0 {
			b.WriteString("\thandle /collect/v1/events* {\n")
			fmt.Fprintf(&b, "\t\treverse_proxy 127.0.0.1:%d\n", opts.TelemetryPort)
			b.WriteString("\t}\n")
		}
		protectedBackends := make(map[string]Backend, len(authPolicies))
		for _, backend := range backends {
			protectedBackends[backend.PathPrefix] = backend
		}
		protectedBackendPaths := map[string]bool{}
		for _, policy := range authPolicies {
			fmt.Fprintf(&b, "\thandle %s {\n", policy.PathPrefix)
			b.WriteString("\t\tbasic_auth {\n")
			fmt.Fprintf(&b, "\t\t\t%s %s\n", policy.Username, policy.PasswordHash)
			b.WriteString("\t\t}\n")
			if backend, ok := protectedBackends[policy.PathPrefix]; ok {
				fmt.Fprintf(&b, "\t\treverse_proxy %s\n", backend.Upstream)
				protectedBackendPaths[policy.PathPrefix] = true
			} else {
				fmt.Fprintf(&b, "\t\troot * %s\n", root)
				b.WriteString("\t\tfile_server\n")
			}
			b.WriteString("\t}\n")
		}
		for _, backend := range backends {
			if protectedBackendPaths[backend.PathPrefix] {
				continue
			}
			fmt.Fprintf(&b, "\thandle %s {\n", backend.PathPrefix)
			fmt.Fprintf(&b, "\t\treverse_proxy %s\n", backend.Upstream)
			b.WriteString("\t}\n")
		}
		b.WriteString("\thandle {\n")
		fmt.Fprintf(&b, "\t\troot * %s\n", root)
		b.WriteString("\t\tfile_server\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func comparePathPrefixSpecificity(a, b string) bool {
	aPrefix := strings.TrimSuffix(strings.TrimSpace(a), "/*")
	bPrefix := strings.TrimSuffix(strings.TrimSpace(b), "/*")
	aDepth := strings.Count(aPrefix, "/")
	bDepth := strings.Count(bPrefix, "/")
	if aDepth != bDepth {
		return aDepth > bDepth
	}
	return a < b
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
