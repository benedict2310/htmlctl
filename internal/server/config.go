package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBindAddr      = "127.0.0.1"
	DefaultPort          = 9400
	DefaultDataDir       = "/var/lib/htmlservd"
	DefaultLogLevel      = "info"
	DefaultCaddyfilePath = "/etc/caddy/Caddyfile"
	DefaultCaddyBinary   = "caddy"
)

type Config struct {
	BindAddr              string    `yaml:"bind"`
	Port                  int       `yaml:"port"`
	DataDir               string    `yaml:"dataDir"`
	LogLevel              string    `yaml:"logLevel"`
	DBPath                string    `yaml:"dbPath"`
	DBWAL                 bool      `yaml:"dbWAL"`
	CaddyfilePath         string    `yaml:"caddyfilePath"`
	CaddyBinaryPath       string    `yaml:"caddyBinaryPath"`
	CaddyConfigBackupPath string    `yaml:"caddyConfigBackupPath"`
	CaddyAutoHTTPS        bool      `yaml:"caddyAutoHTTPS"`
	APIToken              string    `yaml:"apiToken,omitempty"`
	API                   APIConfig `yaml:"api,omitempty"`
}

type APIConfig struct {
	Token string `yaml:"token,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		BindAddr:              DefaultBindAddr,
		Port:                  DefaultPort,
		DataDir:               DefaultDataDir,
		LogLevel:              DefaultLogLevel,
		DBPath:                "",
		DBWAL:                 true,
		CaddyfilePath:         DefaultCaddyfilePath,
		CaddyBinaryPath:       DefaultCaddyBinary,
		CaddyConfigBackupPath: "",
		CaddyAutoHTTPS:        true,
		APIToken:              "",
		API:                   APIConfig{},
	}
}

func LoadConfig(configPath string) (Config, error) {
	cfg := DefaultConfig()

	if strings.TrimSpace(configPath) != "" {
		b, err := os.ReadFile(configPath)
		if err != nil {
			return cfg, fmt.Errorf("read config file %s: %w", configPath, err)
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config file %s: %w", configPath, err)
		}
	}
	cfg.APIToken = strings.TrimSpace(cfg.APIToken)
	cfg.API.Token = strings.TrimSpace(cfg.API.Token)
	if cfg.APIToken == "" {
		cfg.APIToken = cfg.API.Token
	}
	if cfg.API.Token == "" {
		cfg.API.Token = cfg.APIToken
	}

	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_BIND")); v != "" {
		cfg.BindAddr = v
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_PORT")); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("parse HTMLSERVD_PORT=%q: %w", v, err)
		}
		cfg.Port = port
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_DATA_DIR")); v != "" {
		cfg.DataDir = v
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_LOG_LEVEL")); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_DB_PATH")); v != "" {
		cfg.DBPath = v
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_DB_WAL")); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("parse HTMLSERVD_DB_WAL=%q: %w", v, err)
		}
		cfg.DBWAL = parsed
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_CADDYFILE_PATH")); v != "" {
		cfg.CaddyfilePath = v
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_CADDY_BINARY")); v != "" {
		cfg.CaddyBinaryPath = v
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_CADDY_CONFIG_BACKUP")); v != "" {
		cfg.CaddyConfigBackupPath = v
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_CADDY_AUTO_HTTPS")); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("parse HTMLSERVD_CADDY_AUTO_HTTPS=%q: %w", v, err)
		}
		cfg.CaddyAutoHTTPS = parsed
	}
	if v := strings.TrimSpace(os.Getenv("HTMLSERVD_API_TOKEN")); v != "" {
		cfg.APIToken = v
		cfg.API.Token = v
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.BindAddr) == "" {
		return fmt.Errorf("bind address is required")
	}
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("port must be in range 0..65535")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data directory is required")
	}
	if _, err := parseLogLevel(c.LogLevel); err != nil {
		return err
	}
	if strings.TrimSpace(c.APIToken) != "" && strings.TrimSpace(c.API.Token) != "" &&
		strings.TrimSpace(c.APIToken) != strings.TrimSpace(c.API.Token) {
		return fmt.Errorf("apiToken and api.token must match when both are set")
	}
	return nil
}

func (c Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.BindAddr, c.Port)
}
