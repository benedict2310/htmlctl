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

type ServeConfig struct {
	Environment   string
	HTTPAddr      string
	DatabaseURL   string
	PublicBaseURL string
	ResendAPIKey  string
}

type MigrateConfig struct {
	Environment string
	DatabaseURL string
}

func LoadServeFromEnv() (ServeConfig, error) {
	env, err := environmentFromEnv()
	if err != nil {
		return ServeConfig{}, err
	}

	addr := os.Getenv("NEWSLETTER_HTTP_ADDR")
	if strings.TrimSpace(addr) == "" {
		addr = defaultHTTPAddr(env)
	}
	if err := validateLoopbackAddr(addr); err != nil {
		return ServeConfig{}, fmt.Errorf("NEWSLETTER_HTTP_ADDR: %w", err)
	}

	dbURL := strings.TrimSpace(os.Getenv("NEWSLETTER_DATABASE_URL"))
	if dbURL == "" {
		return ServeConfig{}, errors.New("NEWSLETTER_DATABASE_URL is required")
	}

	publicBaseURL := strings.TrimSpace(os.Getenv("NEWSLETTER_PUBLIC_BASE_URL"))
	if publicBaseURL == "" {
		return ServeConfig{}, errors.New("NEWSLETTER_PUBLIC_BASE_URL is required")
	}
	if err := validatePublicBaseURL(publicBaseURL); err != nil {
		return ServeConfig{}, fmt.Errorf("NEWSLETTER_PUBLIC_BASE_URL: %w", err)
	}

	resendAPIKey := strings.TrimSpace(os.Getenv("NEWSLETTER_RESEND_API_KEY"))
	if resendAPIKey == "" {
		return ServeConfig{}, errors.New("NEWSLETTER_RESEND_API_KEY is required")
	}

	return ServeConfig{
		Environment:   env,
		HTTPAddr:      addr,
		DatabaseURL:   dbURL,
		PublicBaseURL: publicBaseURL,
		ResendAPIKey:  resendAPIKey,
	}, nil
}

func LoadMigrateFromEnv() (MigrateConfig, error) {
	env, err := environmentFromEnv()
	if err != nil {
		return MigrateConfig{}, err
	}

	dbURL := strings.TrimSpace(os.Getenv("NEWSLETTER_DATABASE_URL"))
	if dbURL == "" {
		return MigrateConfig{}, errors.New("NEWSLETTER_DATABASE_URL is required")
	}

	return MigrateConfig{Environment: env, DatabaseURL: dbURL}, nil
}

func environmentFromEnv() (string, error) {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("NEWSLETTER_ENV")))
	switch env {
	case "staging", "prod":
		return env, nil
	default:
		return "", errors.New("NEWSLETTER_ENV must be set to staging or prod")
	}
}

func defaultHTTPAddr(environment string) string {
	if environment == "staging" {
		return "127.0.0.1:9501"
	}
	return "127.0.0.1:9502"
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
	if u.Scheme != "https" {
		return errors.New("must use https scheme")
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
