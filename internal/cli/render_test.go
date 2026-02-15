package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderCommandSuccess(t *testing.T) {
	outDir := t.TempDir()
	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"render", "-f", filepath.Join("..", "..", "testdata", "valid-site"), "-o", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "Rendered 1 page(s)") {
		t.Fatalf("expected render summary, got: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Fatalf("expected rendered index.html: %v", err)
	}
}

func TestRenderCommandValidationFailure(t *testing.T) {
	outDir := t.TempDir()
	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"render", "-f", filepath.Join("..", "..", "testdata", "invalid-site"), "-o", outDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "component validation failed") {
		t.Fatalf("expected validation failure error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "pricing") || !strings.Contains(err.Error(), "anchor-id") {
		t.Fatalf("expected pricing anchor-id detail in validation output, got: %v", err)
	}
}

func TestRenderCommandMissingFromFlagShowsUsage(t *testing.T) {
	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"render"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected missing flag error")
	}
	if !strings.Contains(err.Error(), "required flag(s) \"from\" not set") {
		t.Fatalf("expected missing required flag error, got: %v", err)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("expected usage text on stderr, got: %s", errOut.String())
	}
}
