package main

import "testing"

func TestRunUsage(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatal("expected usage error")
	}
	if err := run([]string{"bad"}); err == nil {
		t.Fatal("expected unknown command error")
	}
}
