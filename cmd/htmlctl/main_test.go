package main

import "testing"

func TestRunVersion(t *testing.T) {
	if err := run([]string{"version"}); err != nil {
		t.Fatalf("run(version) error = %v", err)
	}
}

func TestRunRenderMissingFlag(t *testing.T) {
	err := run([]string{"render"})
	if err == nil {
		t.Fatalf("expected render to fail without --from")
	}
}
