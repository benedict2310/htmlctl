package server

import (
	"strings"
	"testing"
)

func TestValidateResourceName(t *testing.T) {
	valid := []string{
		"my-site",
		"site1",
		"production",
		"staging-v2",
		"a",
		"A_1-z",
	}
	for _, name := range valid {
		if err := validateResourceName(name); err != nil {
			t.Fatalf("validateResourceName(%q) error = %v", name, err)
		}
	}

	invalid := []string{
		"",
		"..",
		"../etc",
		"foo/bar",
		"foo bar",
		"foo\nbar",
		"foo{bar}",
		"-starts-with-dash",
		strings.Repeat("a", 129),
	}
	for _, name := range invalid {
		if err := validateResourceName(name); err == nil {
			t.Fatalf("validateResourceName(%q) expected error", name)
		}
	}
}
