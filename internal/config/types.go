package config

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	EnvConfigPath     = "HTMLCTL_CONFIG"
	DefaultAPIVersion = "htmlctl.dev/v1"
	RedactedSecret    = "<redacted>"
)

// Config is the htmlctl CLI configuration file structure.
type Config struct {
	APIVersion     string    `yaml:"apiVersion,omitempty"`
	CurrentContext string    `yaml:"current-context"`
	Contexts       []Context `yaml:"contexts"`
}

// Context defines one named remote target.
type Context struct {
	Name        string `yaml:"name"`
	Server      string `yaml:"server"`
	Website     string `yaml:"website"`
	Environment string `yaml:"environment"`
	Port        int    `yaml:"port,omitempty"`
	Token       string `yaml:"token,omitempty"`
}

// ContextInfo is the resolved context used by command and transport layers.
type ContextInfo struct {
	Name        string
	Server      string
	Website     string
	Environment string
	RemotePort  int
	Token       string
}

func (c *Config) normalize() {
	if strings.TrimSpace(c.APIVersion) == "" {
		c.APIVersion = DefaultAPIVersion
	}
}

// RedactedCopy returns a copy of the config with secret fields masked for safe display.
func (c Config) RedactedCopy() Config {
	clone := c
	if len(c.Contexts) == 0 {
		return clone
	}

	clone.Contexts = make([]Context, len(c.Contexts))
	copy(clone.Contexts, c.Contexts)
	for i := range clone.Contexts {
		clone.Contexts[i].Server = RedactServerURL(clone.Contexts[i].Server)
		if strings.TrimSpace(clone.Contexts[i].Token) != "" {
			clone.Contexts[i].Token = RedactedSecret
		}
	}
	return clone
}

// RedactServerURL masks any password embedded in an ssh:// server URL for safe display.
func RedactServerURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return redactServerURLFallback(raw)
	}
	if u.User != nil {
		if _, ok := u.User.Password(); ok {
			u.User = url.UserPassword(u.User.Username(), RedactedSecret)
		}
	}
	if path := strings.TrimSpace(u.EscapedPath()); path != "" && path != "/" {
		u.Path = "/" + RedactedSecret
		u.RawPath = ""
	}
	if strings.TrimSpace(u.RawQuery) != "" {
		u.RawQuery = "redacted=" + url.QueryEscape(RedactedSecret)
	}
	if strings.TrimSpace(u.Fragment) != "" {
		u.Fragment = RedactedSecret
	}
	return u.String()
}

func redactServerURLFallback(raw string) string {
	const prefix = "ssh://"
	if !strings.HasPrefix(raw, prefix) {
		return raw
	}
	rest := strings.TrimPrefix(raw, prefix)
	fragment := ""
	if index := strings.Index(rest, "#"); index >= 0 {
		fragment = "#" + url.QueryEscape(RedactedSecret)
		rest = rest[:index]
	}
	query := ""
	if index := strings.Index(rest, "?"); index >= 0 {
		query = "?redacted=" + url.QueryEscape(RedactedSecret)
		rest = rest[:index]
	}
	path := ""
	if index := strings.Index(rest, "/"); index >= 0 {
		if rest[index:] == "/" {
			path = "/"
		} else {
			path = "/" + url.PathEscape(RedactedSecret)
		}
		rest = rest[:index]
	}
	at := strings.Index(rest, "@")
	if at >= 0 {
		userInfo := rest[:at]
		if colon := strings.Index(userInfo, ":"); colon >= 0 {
			rest = userInfo[:colon+1] + url.QueryEscape(RedactedSecret) + rest[at:]
		}
	}
	return prefix + rest + path + query + fragment
}

// Validate checks config invariants that must hold for the file to be usable.
func (c Config) Validate() error {
	seen := make(map[string]struct{}, len(c.Contexts))
	for i, ctx := range c.Contexts {
		name := strings.TrimSpace(ctx.Name)
		if name == "" {
			return fmt.Errorf("contexts[%d].name is required", i)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate context name %q", name)
		}
		seen[name] = struct{}{}

		if strings.TrimSpace(ctx.Server) == "" {
			return fmt.Errorf("context %q: server is required", name)
		}
		if strings.TrimSpace(ctx.Website) == "" {
			return fmt.Errorf("context %q: website is required", name)
		}
		if strings.TrimSpace(ctx.Environment) == "" {
			return fmt.Errorf("context %q: environment is required", name)
		}
		if ctx.Port < 0 || ctx.Port > 65535 {
			return fmt.Errorf("context %q: port must be in range 1..65535 (or 0 to use default)", name)
		}
	}

	return nil
}
