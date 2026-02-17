package domain

import "testing"

func TestNormalizeValidDomains(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "futurelab.studio", want: "futurelab.studio"},
		{in: "Staging.FutureLab.Studio", want: "staging.futurelab.studio"},
		{in: "my-site.example.com", want: "my-site.example.com"},
		{in: "  futurelab.studio  ", want: "futurelab.studio"},
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
		"futurelab_.studio",
		"futurelab..studio",
		"futurelab.studio.",
		"-futurelab.studio",
		"futurelab-.studio",
		"127.0.0.1",
	}
	for _, in := range tests {
		if _, err := Normalize(in); err == nil {
			t.Fatalf("Normalize(%q) expected error", in)
		}
	}
}
