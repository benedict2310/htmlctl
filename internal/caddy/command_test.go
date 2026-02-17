package caddy

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestNewExecRunner(t *testing.T) {
	if NewExecRunner() == nil {
		t.Fatalf("expected non-nil exec runner")
	}
}

func TestExecRunnerRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command assumptions are POSIX-specific")
	}
	runner := ExecRunner{}
	stdout, stderr, err := runner.Run(context.Background(), "sh", "-c", "echo ok")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "ok" {
		t.Fatalf("unexpected stdout %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr %q", stderr)
	}
}

func TestExecRunnerRunError(t *testing.T) {
	runner := ExecRunner{}
	_, _, err := runner.Run(context.Background(), "definitely-missing-binary-xyz")
	if err == nil {
		t.Fatalf("expected run error for missing binary")
	}
}
