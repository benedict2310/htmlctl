package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultMaxBodyBytes = 32 << 10
	defaultMaxEvents    = 10
)

type ServeConfig struct {
	Environment      string
	HTTPAddr         string
	PublicBaseURL    string
	HTMLSERVDBaseURL string
	HTMLSERVDToken   string
	AllowedEvents    []string
	MaxBodyBytes     int
	MaxEvents        int
}

func LoadServeFromEnv() (ServeConfig, error) {
	env, err := environmentFromEnv()
	if err != nil {
		return ServeConfig{}, err
	}

	addr := strings.TrimSpace(os.Getenv("TELEMETRY_COLLECTOR_HTTP_ADDR"))
	if addr == "" {
		addr = defaultHTTPAddr(env)
	}
	if err := validateLoopbackAddr(addr); err != nil {
		return ServeConfig{}, fmt.Errorf("TELEMETRY_COLLECTOR_HTTP_ADDR: %w", err)
	}

	publicBaseURL := strings.TrimSpace(os.Getenv("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL"))
	if publicBaseURL == "" {
		return ServeConfig{}, errors.New("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL is required")
	}
	if err := validatePublicBaseURL(publicBaseURL); err != nil {
		return ServeConfig{}, fmt.Errorf("TELEMETRY_COLLECTOR_PUBLIC_BASE_URL: %w", err)
	}

	htmlservdBaseURL := strings.TrimSpace(os.Getenv("TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL"))
	if htmlservdBaseURL == "" {
		htmlservdBaseURL = "http://127.0.0.1:9400"
	}
	if err := validateHTMLSERVDBaseURL(htmlservdBaseURL); err != nil {
		return ServeConfig{}, fmt.Errorf("TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL: %w", err)
	}

	token := strings.TrimSpace(os.Getenv("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN"))
	if token == "" {
		return ServeConfig{}, errors.New("TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN is required")
	}

	allowedEvents, err := parseAllowedEvents(strings.TrimSpace(os.Getenv("TELEMETRY_COLLECTOR_ALLOWED_EVENTS")))
	if err != nil {
		return ServeConfig{}, fmt.Errorf("TELEMETRY_COLLECTOR_ALLOWED_EVENTS: %w", err)
	}

	maxBodyBytes, err := parsePositiveInt("TELEMETRY_COLLECTOR_MAX_BODY_BYTES", defaultMaxBodyBytes)
	if err != nil {
		return ServeConfig{}, err
	}
	maxEvents, err := parsePositiveInt("TELEMETRY_COLLECTOR_MAX_EVENTS", defaultMaxEvents)
	if err != nil {
		return ServeConfig{}, err
	}

	return ServeConfig{
		Environment:      env,
		HTTPAddr:         addr,
		PublicBaseURL:    publicBaseURL,
		HTMLSERVDBaseURL: htmlservdBaseURL,
		HTMLSERVDToken:   token,
		AllowedEvents:    allowedEvents,
		MaxBodyBytes:     maxBodyBytes,
		MaxEvents:        maxEvents,
	}, nil
}

func environmentFromEnv() (string, error) {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("TELEMETRY_COLLECTOR_ENV")))
	switch env {
	case "staging", "prod":
		return env, nil
	default:
		return "", errors.New("TELEMETRY_COLLECTOR_ENV must be set to staging or prod")
	}
}

func defaultHTTPAddr(environment string) string {
	if environment == "staging" {
		return "127.0.0.1:9601"
	}
	return "127.0.0.1:9602"
}

func parsePositiveInt(name string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func parseAllowedEvents(raw string) ([]string, error) {
	if raw == "" {
		return []string{"page_view", "link_click", "cta_click", "newsletter_signup"}, nil
	}
	parts := strings.Split(raw, ",")
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, errors.New("must not contain empty values")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func validateLoopbackAddr(addr string) error {
	host, portRaw, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid host:port: %w", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("port must be numeric and between 1 and 65535")
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return errors.New("host must be localhost or loopback IP")
	}
	if !ip.IsLoopback() {
		return errors.New("host must be loopback only")
	}
	return nil
}

func validatePublicBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("must use http or https scheme")
	}
	if u.Hostname() == "" {
		return errors.New("hostname is required")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return errors.New("port must be between 1 and 65535")
		}
	}
	if u.Path != "" && u.Path != "/" {
		return errors.New("must not include a path")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("must not include query or fragment")
	}
	if u.User != nil {
		return errors.New("must not contain userinfo")
	}
	return nil
}

func validateHTMLSERVDBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" {
		return errors.New("must use http scheme")
	}
	if u.Hostname() == "" {
		return errors.New("hostname is required")
	}
	if u.Path != "" && u.Path != "/" {
		return errors.New("must not include a path")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("must not include query or fragment")
	}
	if u.User != nil {
		return errors.New("must not contain userinfo")
	}
	if hostname := strings.TrimSpace(u.Hostname()); hostname != "localhost" {
		ip := net.ParseIP(hostname)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("host must be localhost or loopback IP")
		}
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return errors.New("port must be between 1 and 65535")
		}
	}
	return nil
}
