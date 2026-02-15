package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandNoArgsPrintsUsage(t *testing.T) {
	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "Usage:") {
		t.Fatalf("expected usage output, got: %s", help)
	}
	for _, sub := range []string{"render", "serve", "version"} {
		if !strings.Contains(help, sub) {
			t.Fatalf("expected help output to include %q", sub)
		}
	}
}
