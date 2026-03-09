package server

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(2, time.Minute, func() time.Time { return now })
	if !limiter.Allow("ip") {
		t.Fatal("expected first request to pass")
	}
	if !limiter.Allow("ip") {
		t.Fatal("expected second request to pass")
	}
	if limiter.Allow("ip") {
		t.Fatal("expected third request to be rejected")
	}
	now = now.Add(2 * time.Minute)
	if !limiter.Allow("ip") {
		t.Fatal("expected request after window reset to pass")
	}
}
