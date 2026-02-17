package caddy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, stderr string, err error)
}

type ExecRunner struct{}

func NewExecRunner() CommandRunner {
	return ExecRunner{}
}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("execute %s %s: %w", name, strings.Join(args, " "), err)
	}
	return stdout.String(), stderr.String(), nil
}
