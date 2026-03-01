package backend

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

const maxPathPrefixLength = 256

func ValidatePathPrefix(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("path prefix is required")
	}
	if len(value) > maxPathPrefixLength {
		return "", fmt.Errorf("path prefix exceeds maximum length of %d characters", maxPathPrefixLength)
	}
	if strings.ContainsAny(value, "?#") {
		return "", fmt.Errorf("path prefix must not contain a query string or fragment")
	}
	if !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("path prefix must start with /")
	}
	if !strings.HasSuffix(value, "/*") {
		return "", fmt.Errorf("path prefix must use the canonical /* suffix")
	}
	if strings.Contains(value, "..") {
		return "", fmt.Errorf("path prefix must not contain ..")
	}

	prefix := strings.TrimSuffix(value, "/*")
	if prefix == "" || prefix == "/" {
		return "", fmt.Errorf("path prefix must include at least one non-empty path segment")
	}
	if cleaned := path.Clean(prefix); cleaned != prefix {
		return "", fmt.Errorf("path prefix must not contain empty or non-canonical path segments")
	}

	return value, nil
}

func ValidateUpstreamURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("upstream URL is required")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid upstream URL: %w", err)
	}
	if !parsed.IsAbs() {
		return "", fmt.Errorf("upstream URL must be absolute")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("upstream URL must use http or https")
	}
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("upstream URL host is required")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("upstream URL must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return "", fmt.Errorf("upstream URL must not include a query string")
	}
	if parsed.Fragment != "" {
		return "", fmt.Errorf("upstream URL must not include a fragment")
	}

	return value, nil
}
