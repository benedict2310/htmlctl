package backend

import (
	"strings"
	"testing"
)

func TestValidatePathPrefix(t *testing.T) {
	longPrefix := "/" + strings.Repeat("a", maxPathPrefixLength-3) + "/*"
	tooLongPrefix := "/" + strings.Repeat("a", maxPathPrefixLength-2) + "/*"

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "api", input: "/api/*", want: "/api/*"},
		{name: "nested", input: "/api/v1/*", want: "/api/v1/*"},
		{name: "trimmed", input: " /auth/callback/* ", want: "/auth/callback/*"},
		{name: "max length", input: longPrefix, want: longPrefix},
		{name: "empty", input: "", wantErr: "required"},
		{name: "root", input: "/", wantErr: "canonical /* suffix"},
		{name: "missing slash", input: "api/*", wantErr: "must start with /"},
		{name: "ambiguous trailing slash", input: "/api/", wantErr: "canonical /* suffix"},
		{name: "missing wildcard", input: "/api", wantErr: "canonical /* suffix"},
		{name: "double wildcard", input: "/api/**", wantErr: "canonical /* suffix"},
		{name: "query", input: "/api/*?x=1", wantErr: "query string or fragment"},
		{name: "fragment", input: "/api/*#frag", wantErr: "query string or fragment"},
		{name: "parent traversal", input: "/../api/*", wantErr: "must not contain .."},
		{name: "double slash", input: "/api//v1/*", wantErr: "non-canonical"},
		{name: "dot segment", input: "/api/./v1/*", wantErr: "non-canonical"},
		{name: "too long", input: tooLongPrefix, wantErr: "maximum length"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePathPrefix(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePathPrefix() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ValidatePathPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateUpstreamURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "https", input: "https://api.example.com", want: "https://api.example.com"},
		{name: "uppercase scheme", input: "HTTPS://api.example.com", want: "HTTPS://api.example.com"},
		{name: "http localhost", input: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "path base", input: " https://auth.internal.example.com/base ", want: "https://auth.internal.example.com/base"},
		{name: "empty", input: "", wantErr: "required"},
		{name: "relative", input: "api.example.com", wantErr: "absolute"},
		{name: "ftp", input: "ftp://api.example.com", wantErr: "http or https"},
		{name: "credentials", input: "https://user:pass@example.com", wantErr: "credentials"},
		{name: "bare query marker", input: "https://api.example.com?", wantErr: "query string"},
		{name: "query", input: "https://api.example.com?debug=1", wantErr: "query string"},
		{name: "fragment", input: "https://api.example.com#frag", wantErr: "fragment"},
		{name: "host required", input: "https:///path-only", wantErr: "host is required"},
		{name: "missing host with port only", input: "https://:443", wantErr: "host is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateUpstreamURL(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateUpstreamURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ValidateUpstreamURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
