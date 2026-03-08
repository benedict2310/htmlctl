package links

import (
	"strings"
	"testing"
)

func TestGenerateAndParseUnsubscribeToken(t *testing.T) {
	token := GenerateUnsubscribeToken("secret", 42)
	id, err := ParseUnsubscribeToken("secret", token)
	if err != nil {
		t.Fatalf("ParseUnsubscribeToken returned error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected subscriber id 42, got %d", id)
	}
}

func TestParseUnsubscribeTokenRejectsWrongSecret(t *testing.T) {
	token := GenerateUnsubscribeToken("secret", 42)
	if _, err := ParseUnsubscribeToken("other", token); err == nil {
		t.Fatal("expected wrong secret to fail")
	}
}

func TestUnsubscribeURL(t *testing.T) {
	url := UnsubscribeURL("https://example.com/", "secret", 7)
	if !strings.HasPrefix(url, "https://example.com/newsletter/unsubscribe?token=") {
		t.Fatalf("unexpected url: %s", url)
	}
}
