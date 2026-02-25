package domain

import "testing"

func TestNormalizeValidDomains(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "example.com", want: "example.com"},
		{in: "Staging.Example.Com", want: "staging.example.com"},
		{in: "my-site.example.com", want: "my-site.example.com"},
		{in: "  example.com  ", want: "example.com"},
	}
	for _, tc := range tests {
		got, err := Normalize(tc.in)
		if err != nil {
			t.Fatalf("Normalize(%q) error = %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeInvalidDomains(t *testing.T) {
	tests := []string{
		"",
		" ",
		"localhost",
		"not a domain",
		"example_.com",
		"example..com",
		"example.com.",
		"-example.com",
		"example-.com",
		"127.0.0.1",
	}
	for _, in := range tests {
		if _, err := Normalize(in); err == nil {
			t.Fatalf("Normalize(%q) expected error", in)
		}
	}
}
