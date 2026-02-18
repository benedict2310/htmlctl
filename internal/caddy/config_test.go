package caddy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateConfigSorted(t *testing.T) {
	cfg, err := GenerateConfig([]Site{
		{Domain: "staging.futurelab.studio", Root: "/srv/futurelab/staging/current"},
		{Domain: "futurelab.studio", Root: "/srv/futurelab/prod/current"},
	})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}
	first := strings.Index(cfg, "futurelab.studio {")
	second := strings.Index(cfg, "staging.futurelab.studio {")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("expected deterministic sorted output, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "\troot * /srv/futurelab/prod/current") {
		t.Fatalf("expected prod root path in config, got:\n%s", cfg)
	}
}

func TestGenerateConfigEmpty(t *testing.T) {
	cfg, err := GenerateConfig(nil)
	if err != nil {
		t.Fatalf("GenerateConfig(nil) error = %v", err)
	}
	if strings.TrimSpace(cfg) != "# managed by htmlservd" {
		t.Fatalf("unexpected empty config output: %q", cfg)
	}
}

func TestGenerateConfigRejectsInvalidSite(t *testing.T) {
	if _, err := GenerateConfig([]Site{{Domain: "", Root: "/x"}}); err == nil {
		t.Fatalf("expected missing-domain error")
	}
	if _, err := GenerateConfig([]Site{{Domain: "futurelab.studio", Root: ""}}); err == nil {
		t.Fatalf("expected missing-root error")
	}
}

func TestGenerateConfigWithAutoHTTPSDisabled(t *testing.T) {
	cfg, err := GenerateConfigWithOptions([]Site{
		{Domain: "futurelab.studio", Root: "/srv/futurelab/prod/current"},
	}, ConfigOptions{DisableAutoHTTPS: true})
	if err != nil {
		t.Fatalf("GenerateConfigWithOptions() error = %v", err)
	}
	if !strings.Contains(cfg, "auto_https off") {
		t.Fatalf("expected auto_https off in config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "http://futurelab.studio {") {
		t.Fatalf("expected explicit http site address in config, got:\n%s", cfg)
	}
}

func TestWriteConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Caddyfile")
	if err := WriteConfig(path, "futurelab.studio {\n\tfile_server\n}\n"); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(b), "futurelab.studio") {
		t.Fatalf("unexpected config content: %s", string(b))
	}
}

func TestWriteConfigEmptyPath(t *testing.T) {
	if err := WriteConfig("", "x"); err == nil {
		t.Fatalf("expected empty path error")
	}
}

func TestWriteConfigCreateDirFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	path := filepath.Join(blocker, "Caddyfile")
	if err := WriteConfig(path, "x"); err == nil {
		t.Fatalf("expected directory creation failure")
	}
}
