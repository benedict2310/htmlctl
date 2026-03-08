package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benedict2310/htmlctl/extensions/newsletter/service/internal/links"
)

func TestHealthz(t *testing.T) {
	ts := httptest.NewServer(New(Options{}))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSignupAcceptedAndGeneric(t *testing.T) {
	var mailCalls int32
	store := stubStore{
		signupFn: func(_ context.Context, email string, tokenHash []byte, expiresAt, now time.Time) (bool, error) {
			if email != "test@example.com" {
				t.Fatalf("unexpected normalized email: %q", email)
			}
			if len(tokenHash) == 0 {
				t.Fatal("expected token hash")
			}
			if !expiresAt.After(now) {
				t.Fatal("expected expiry after now")
			}
			return true, nil
		},
	}
	mailer := stubMailer{
		sendFn: func(_ context.Context, email, verifyURL string) error {
			atomic.AddInt32(&mailCalls, 1)
			if email != "test@example.com" {
				t.Fatalf("unexpected mailer email: %q", email)
			}
			if !strings.HasPrefix(verifyURL, "https://staging.example.com/newsletter/verify?token=") {
				t.Fatalf("unexpected verify URL: %s", verifyURL)
			}
			return nil
		},
	}

	ts := httptest.NewServer(New(Options{
		Store:         store,
		Mailer:        mailer,
		Environment:   "staging",
		PublicBaseURL: "https://staging.example.com",
		Now: func() time.Time {
			return time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
		},
	}))
	t.Cleanup(ts.Close)

	resp, err := http.Post(ts.URL+"/newsletter/signup", "application/json", strings.NewReader(`{"email":" Test@Example.com "}`))
	if err != nil {
		t.Fatalf("POST /newsletter/signup: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var body signupResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", body.Status)
	}
	if atomic.LoadInt32(&mailCalls) != 1 {
		t.Fatalf("expected 1 mail call, got %d", mailCalls)
	}
}

func TestSignupInvalidEmail(t *testing.T) {
	ts := httptest.NewServer(New(Options{Store: stubStore{}}))
	t.Cleanup(ts.Close)

	resp, err := http.Post(ts.URL+"/newsletter/signup", "application/json", strings.NewReader(`{"email":"not-an-email"}`))
	if err != nil {
		t.Fatalf("POST /newsletter/signup: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSignupRateLimit(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		Store: stubStore{
			signupFn: func(context.Context, string, []byte, time.Time, time.Time) (bool, error) {
				return false, nil
			},
		},
	}))
	t.Cleanup(ts.Close)

	client := &http.Client{}
	for i := 0; i < defaultSignupPerMinute; i++ {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/newsletter/signup", strings.NewReader(`{"email":"x@example.com"}`))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.10:1234"
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("request %d expected 202, got %d", i, resp.StatusCode)
		}
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/newsletter/signup", strings.NewReader(`{"email":"x@example.com"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:1234"
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("rate-limit request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
}

func TestVerifySuccess(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		Store: stubStore{
			verifyFn: func(_ context.Context, tokenHash []byte, _ time.Time) (VerifyResult, error) {
				if len(tokenHash) == 0 {
					t.Fatal("expected token hash")
				}
				return VerifyConfirmed, nil
			},
		},
	}))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/newsletter/verify?token=abc")
	if err != nil {
		t.Fatalf("GET /newsletter/verify: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	if body.Status != "confirmed" {
		t.Fatalf("expected confirmed status, got %q", body.Status)
	}
}

func TestVerifyInvalidToken(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		Store: stubStore{
			verifyFn: func(context.Context, []byte, time.Time) (VerifyResult, error) {
				return VerifyInvalid, errInvalidToken
			},
		},
	}))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/newsletter/verify?token=abc")
	if err != nil {
		t.Fatalf("GET /newsletter/verify: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestVerifyHTMLResponseForBrowserAccept(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		Store: stubStore{
			verifyFn: func(context.Context, []byte, time.Time) (VerifyResult, error) {
				return VerifyConfirmed, nil
			},
		},
		PublicBaseURL: "https://example.com",
	}))
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/newsletter/verify?token=abc", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /newsletter/verify html: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("expected html content-type, got %q", got)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read html response: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "Email confirmed") {
		t.Fatalf("expected confirmation title in html body, got: %s", body)
	}
	if !strings.Contains(body, `href="https://example.com"`) {
		t.Fatalf("expected site CTA in html body, got: %s", body)
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		Store: stubStore{
			verifyFn: func(context.Context, []byte, time.Time) (VerifyResult, error) {
				return VerifyExpired, errExpiredToken
			},
		},
	}))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/newsletter/verify?token=abc")
	if err != nil {
		t.Fatalf("GET /newsletter/verify: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusGone {
		t.Fatalf("expected 410, got %d", resp.StatusCode)
	}
}

func TestUnsubscribeSuccess(t *testing.T) {
	token := links.GenerateUnsubscribeToken("secret", 9)
	ts := httptest.NewServer(New(Options{
		Store: stubStore{
			unsubscribeFn: func(_ context.Context, subscriberID int64, _ time.Time) (UnsubscribeResult, error) {
				if subscriberID != 9 {
					t.Fatalf("unexpected subscriber id: %d", subscriberID)
				}
				return UnsubscribeConfirmed, nil
			},
		},
		LinkSecret: "secret",
	}))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/newsletter/unsubscribe?token=" + token)
	if err != nil {
		t.Fatalf("GET /newsletter/unsubscribe: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUnsubscribeInvalidToken(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		Store:      stubStore{},
		LinkSecret: "secret",
	}))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/newsletter/unsubscribe?token=bad-token")
	if err != nil {
		t.Fatalf("GET /newsletter/unsubscribe: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestNewsletterUnknownPath(t *testing.T) {
	ts := httptest.NewServer(New(Options{Store: stubStore{}}))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/newsletter/unknown")
	if err != nil {
		t.Fatalf("GET /newsletter/unknown: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestClientIP_UntrustedRemoteIgnoresForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/newsletter/signup", nil)
	req.RemoteAddr = "198.51.100.9:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.8, 203.0.113.9")

	got := clientIP(req)
	if got != "198.51.100.9" {
		t.Fatalf("expected remote addr IP, got %q", got)
	}
}

func TestClientIP_TrustedProxyUsesRightMostForwardedIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/newsletter/signup", nil)
	req.RemoteAddr = "127.0.0.1:4321"
	req.Header.Set("X-Forwarded-For", "198.51.100.2, 203.0.113.7")

	got := clientIP(req)
	if got != "203.0.113.7" {
		t.Fatalf("expected right-most forwarded IP, got %q", got)
	}
}

func TestRateLimiter_BoundsCardinality(t *testing.T) {
	now := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(1, time.Minute, func() time.Time { return now })
	limiter.maxEntries = 2
	limiter.sweepEveryN = 0

	if !limiter.Allow("a") || !limiter.Allow("b") {
		t.Fatal("expected first two keys to pass")
	}
	if limiter.Allow("c") {
		t.Fatal("expected third unique key to be rejected when map is full")
	}
}

func TestRightMostValidForwardedIP_ParsesAndNormalizes(t *testing.T) {
	ip, ok := rightMostValidForwardedIP("garbage, 2001:db8::1")
	if !ok {
		t.Fatal("expected parse success")
	}
	if netip.MustParseAddr(ip).String() != "2001:db8::1" {
		t.Fatalf("unexpected normalized ip: %q", ip)
	}
}

type stubStore struct {
	signupFn      func(ctx context.Context, email string, tokenHash []byte, expiresAt, now time.Time) (bool, error)
	verifyFn      func(ctx context.Context, tokenHash []byte, now time.Time) (VerifyResult, error)
	unsubscribeFn func(ctx context.Context, subscriberID int64, now time.Time) (UnsubscribeResult, error)
}

func (s stubStore) Signup(ctx context.Context, email string, tokenHash []byte, expiresAt, now time.Time) (bool, error) {
	if s.signupFn == nil {
		return false, nil
	}
	return s.signupFn(ctx, email, tokenHash, expiresAt, now)
}

func (s stubStore) Verify(ctx context.Context, tokenHash []byte, now time.Time) (VerifyResult, error) {
	if s.verifyFn == nil {
		return VerifyInvalid, errors.New("not found")
	}
	return s.verifyFn(ctx, tokenHash, now)
}

func (s stubStore) Unsubscribe(ctx context.Context, subscriberID int64, now time.Time) (UnsubscribeResult, error) {
	if s.unsubscribeFn == nil {
		return UnsubscribeInvalid, errInvalidUnsubscribeToken
	}
	return s.unsubscribeFn(ctx, subscriberID, now)
}

type stubMailer struct {
	sendFn func(ctx context.Context, email, verifyURL string) error
}

func (m stubMailer) SendVerification(ctx context.Context, email, verifyURL string) error {
	if m.sendFn == nil {
		return nil
	}
	return m.sendFn(ctx, email, verifyURL)
}
