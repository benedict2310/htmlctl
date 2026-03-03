package authpolicy

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestValidateUsername(t *testing.T) {
	longValue := strings.Repeat("a", MaxUsernameLength+1)
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "basic", input: "reviewer", want: "reviewer"},
		{name: "trimmed", input: " reviewer ", want: "reviewer"},
		{name: "empty", input: "", wantErr: "required"},
		{name: "space", input: "review user", wantErr: "printable ASCII"},
		{name: "unicode", input: "r\xe9viewer", wantErr: "printable ASCII"},
		{name: "colon", input: "reviewer:ops", wantErr: "must not contain ':'"},
		{name: "caddy braces", input: "reviewer}", wantErr: "unsupported Caddyfile characters"},
		{name: "caddy quote", input: `review"er`, wantErr: "unsupported Caddyfile characters"},
		{name: "caddy comment", input: "#reviewer", wantErr: "unsupported Caddyfile characters"},
		{name: "too long", input: longValue, wantErr: "maximum length"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateUsername(tt.input)
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
				t.Fatalf("ValidateUsername() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ValidateUsername() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePasswordHash(t *testing.T) {
	goodHash, err := bcrypt.GenerateFromPassword([]byte("secret-password"), MinBcryptCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	lowCostHash, err := bcrypt.GenerateFromPassword([]byte("secret-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword(min) error = %v", err)
	}
	highCostHash, err := bcrypt.GenerateFromPassword([]byte("secret-password"), MaxBcryptCost+1)
	if err != nil {
		t.Fatalf("GenerateFromPassword(high) error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "valid", input: string(goodHash)},
		{name: "empty", input: "", wantErr: "required"},
		{name: "invalid", input: "not-bcrypt", wantErr: "valid bcrypt"},
		{name: "low cost", input: string(lowCostHash), wantErr: "between"},
		{name: "high cost", input: string(highCostHash), wantErr: "between"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePasswordHash(tt.input)
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
				t.Fatalf("ValidatePasswordHash() error = %v", err)
			}
			if got != tt.input {
				t.Fatalf("ValidatePasswordHash() = %q, want %q", got, tt.input)
			}
		})
	}
}
