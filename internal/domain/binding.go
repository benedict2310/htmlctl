package domain

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var labelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type Binding struct {
	ID          int64  `json:"id" yaml:"id"`
	Domain      string `json:"domain" yaml:"domain"`
	Website     string `json:"website" yaml:"website"`
	Environment string `json:"environment" yaml:"environment"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string `json:"updatedAt" yaml:"updatedAt"`
}

func Normalize(value string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(value))
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	if strings.HasSuffix(domain, ".") {
		return "", fmt.Errorf("domain must not have a trailing dot")
	}
	if len(domain) > 253 {
		return "", fmt.Errorf("domain exceeds maximum length of 253 characters")
	}
	if ip := net.ParseIP(domain); ip != nil {
		return "", fmt.Errorf("domain must be a hostname, not an IP address")
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain must include at least one dot")
	}
	for _, label := range labels {
		if label == "" {
			return "", fmt.Errorf("domain contains an empty label")
		}
		if !labelPattern.MatchString(label) {
			return "", fmt.Errorf("invalid domain label %q", label)
		}
	}
	return domain, nil
}
