package main

import "testing"

func TestHasRealResendKey(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		valid bool
	}{
		{name: "empty", key: "", valid: false},
		{name: "placeholder", key: "REPLACE_WITH_STAGING_RESEND_API_KEY", valid: false},
		{name: "whitespace placeholder", key: "  replace_with_key  ", valid: false},
		{name: "real", key: "re_123", valid: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasRealResendKey(tc.key); got != tc.valid {
				t.Fatalf("hasRealResendKey(%q) = %v, want %v", tc.key, got, tc.valid)
			}
		})
	}
}

func TestEnsureMailerConfig(t *testing.T) {
	if err := ensureMailerConfig("prod", false); err == nil {
		t.Fatal("expected prod without mailer key to fail")
	}
	if err := ensureMailerConfig("prod", true); err != nil {
		t.Fatalf("expected prod with mailer key to pass: %v", err)
	}
	if err := ensureMailerConfig("staging", false); err != nil {
		t.Fatalf("expected staging without key to pass: %v", err)
	}
}
