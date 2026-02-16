package cli

import (
	"errors"
	"testing"
)

func TestExitCode(t *testing.T) {
	if got := ExitCode(nil); got != 0 {
		t.Fatalf("ExitCode(nil) = %d, want 0", got)
	}
	if got := ExitCode(errors.New("boom")); got != 1 {
		t.Fatalf("ExitCode(plain error) = %d, want 1", got)
	}
	if got := ExitCode(exitCodeError(2, errors.New("boom"))); got != 2 {
		t.Fatalf("ExitCode(exitCodeError(2)) = %d, want 2", got)
	}
}
