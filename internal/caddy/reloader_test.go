package caddy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRunner struct {
	mu    sync.Mutex
	calls []string
	fn    func(name string, args ...string) (string, string, error)
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	f.mu.Unlock()
	if f.fn != nil {
		return f.fn(name, args...)
	}
	return "", "", nil
}

func TestReloaderSuccess(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "Caddyfile")
	backupPath := filepath.Join(dir, "Caddyfile.bak")
	if err := os.WriteFile(configPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old config: %v", err)
	}

	runner := &fakeRunner{}
	reloader, err := NewReloader("caddy", configPath, backupPath, func(ctx context.Context) (string, error) {
		return "futurelab.studio {\n\troot * /srv/futurelab/prod/current\n\tfile_server\n}\n", nil
	}, runner)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	if err := reloader.Reload(context.Background(), "test"); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if !strings.Contains(string(b), "futurelab.studio") {
		t.Fatalf("expected updated config, got: %s", string(b))
	}
	bak, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup config: %v", err)
	}
	if string(bak) != "old" {
		t.Fatalf("unexpected backup content: %q", string(bak))
	}
}

func TestReloaderValidateFailureKeepsCurrentConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "Caddyfile")
	if err := os.WriteFile(configPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old config: %v", err)
	}

	runner := &fakeRunner{
		fn: func(name string, args ...string) (string, string, error) {
			if len(args) > 0 && args[0] == "validate" {
				return "", "invalid config", fmt.Errorf("exit status 1")
			}
			return "", "", nil
		},
	}
	reloader, err := NewReloader("caddy", configPath, "", func(ctx context.Context) (string, error) {
		return "broken", nil
	}, runner)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	if err := reloader.Reload(context.Background(), "test"); err == nil {
		t.Fatalf("expected validate failure")
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after failure: %v", err)
	}
	if string(b) != "old" {
		t.Fatalf("config should remain unchanged, got: %q", string(b))
	}
}

func TestReloaderReloadFailureRestoresBackup(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "Caddyfile")
	backupPath := filepath.Join(dir, "Caddyfile.bak")
	if err := os.WriteFile(configPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old config: %v", err)
	}

	runner := &fakeRunner{
		fn: func(name string, args ...string) (string, string, error) {
			if len(args) > 0 && args[0] == "reload" {
				return "", "reload failed", fmt.Errorf("exit status 1")
			}
			return "", "", nil
		},
	}
	reloader, err := NewReloader("caddy", configPath, backupPath, func(ctx context.Context) (string, error) {
		return "futurelab.studio {\n\tfile_server\n}\n", nil
	}, runner)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	if err := reloader.Reload(context.Background(), "test"); err == nil {
		t.Fatalf("expected reload failure")
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after restore: %v", err)
	}
	if string(b) != "old" {
		t.Fatalf("expected config restored to old content, got %q", string(b))
	}
}

func TestReloaderSerializesConcurrentCalls(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "Caddyfile")

	var active int32
	var maxActive int32
	runner := &fakeRunner{}
	reloader, err := NewReloader("caddy", configPath, "", func(ctx context.Context) (string, error) {
		current := atomic.AddInt32(&active, 1)
		for {
			oldMax := atomic.LoadInt32(&maxActive)
			if current <= oldMax {
				break
			}
			if atomic.CompareAndSwapInt32(&maxActive, oldMax, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return "futurelab.studio {\n\tfile_server\n}\n", nil
	}, runner)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	errCh := make(chan error, 2)
	go func() {
		defer wg.Done()
		errCh <- reloader.Reload(context.Background(), "one")
	}()
	go func() {
		defer wg.Done()
		errCh <- reloader.Reload(context.Background(), "two")
	}()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("Reload() error = %v", err)
		}
	}
	if atomic.LoadInt32(&maxActive) != 1 {
		t.Fatalf("expected serialized reloads, max concurrent=%d", atomic.LoadInt32(&maxActive))
	}
}

func TestNewReloaderValidationAndDefaults(t *testing.T) {
	_, err := NewReloader("caddy", "/tmp/Caddyfile", "", nil, nil)
	if err == nil {
		t.Fatalf("expected nil generator error")
	}
	reloader, err := NewReloader("", "/tmp/Caddyfile", "", func(ctx context.Context) (string, error) {
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("NewReloader() unexpected error = %v", err)
	}
	if reloader.BinaryPath != "caddy" {
		t.Fatalf("expected default binary path, got %q", reloader.BinaryPath)
	}
	if reloader.BackupPath != "/tmp/Caddyfile.bak" {
		t.Fatalf("expected default backup path, got %q", reloader.BackupPath)
	}
}

func TestReloaderReloadFailureWithoutPreviousConfigRemovesNewConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "Caddyfile")
	runner := &fakeRunner{
		fn: func(name string, args ...string) (string, string, error) {
			if len(args) > 0 && args[0] == "reload" {
				return "", "reload failed", fmt.Errorf("exit status 1")
			}
			return "", "", nil
		},
	}
	reloader, err := NewReloader("caddy", configPath, "", func(ctx context.Context) (string, error) {
		return "futurelab.studio {\n\tfile_server\n}\n", nil
	}, runner)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	if err := reloader.Reload(context.Background(), "test"); err == nil {
		t.Fatalf("expected reload failure")
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected config removed after failed reload without previous config, stat err=%v", err)
	}
}

func TestCopyFileError(t *testing.T) {
	if err := copyFile("/definitely/missing/source", "/tmp/unused"); err == nil {
		t.Fatalf("expected copyFile error for missing source")
	}
}

func TestReloaderGeneratorError(t *testing.T) {
	reloader, err := NewReloader("caddy", filepath.Join(t.TempDir(), "Caddyfile"), "", func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}, &fakeRunner{})
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}
	if err := reloader.Reload(context.Background(), "test"); err == nil {
		t.Fatalf("expected generator error")
	}
}

func TestReloaderWriteTempConfigFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	reloader, err := NewReloader("caddy", filepath.Join(blocker, "Caddyfile"), "", func(ctx context.Context) (string, error) {
		return "futurelab.studio {\n\tfile_server\n}\n", nil
	}, &fakeRunner{})
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}
	if err := reloader.Reload(context.Background(), "test"); err == nil {
		t.Fatalf("expected temporary config write failure")
	}
}

func TestReloaderBackupFailure(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "Caddyfile")
	if err := os.WriteFile(configPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	reloader, err := NewReloader("caddy", configPath, filepath.Join(blocker, "backup"), func(ctx context.Context) (string, error) {
		return "futurelab.studio {\n\tfile_server\n}\n", nil
	}, &fakeRunner{})
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}
	if err := reloader.Reload(context.Background(), "test"); err == nil {
		t.Fatalf("expected backup failure")
	}
}

func TestRestoreBackupMissingBackupFile(t *testing.T) {
	reloader, err := NewReloader("caddy", filepath.Join(t.TempDir(), "Caddyfile"), "", func(ctx context.Context) (string, error) {
		return "", nil
	}, &fakeRunner{})
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}
	if err := reloader.restoreBackup(true); err == nil {
		t.Fatalf("expected restore backup error")
	}
}
