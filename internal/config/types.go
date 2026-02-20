package config

import (
	"fmt"
	"strings"
)

const (
	EnvConfigPath     = "HTMLCTL_CONFIG"
	DefaultAPIVersion = "htmlctl.dev/v1"
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
