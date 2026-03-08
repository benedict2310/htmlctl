package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/extensions/newsletter/service/internal/links"
)

const (
	defaultSignupPerMinute = 8
	defaultVerifyPerMinute = 20
	defaultUnsubPerMinute  = 20
	defaultTokenTTL        = 24 * time.Hour
	maxJSONBodyBytes       = 8 << 10
)

var (
	errInvalidToken            = errors.New("invalid token")
	errExpiredToken            = errors.New("expired token")
	errInvalidUnsubscribeToken = errors.New("invalid unsubscribe token")
)

type signupRequest struct {
	Email string `json:"email"`
}

type apiError struct {
	Error string `json:"error"`
}

type signupResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type verifyResponse struct {
	Status string `json:"status"`
}

type Clock func() time.Time

type Store interface {
	Signup(ctx context.Context, email string, tokenHash []byte, expiresAt, now time.Time) (bool, error)
	Verify(ctx context.Context, tokenHash []byte, now time.Time) (VerifyResult, error)
	Unsubscribe(ctx context.Context, subscriberID int64, now time.Time) (UnsubscribeResult, error)
}

type VerifyResult int

const (
	VerifyInvalid VerifyResult = iota
	VerifyExpired
	VerifyAlreadyUsed
	VerifyConfirmed
)

type UnsubscribeResult int

const (
	UnsubscribeInvalid UnsubscribeResult = iota
	UnsubscribeAlreadyUnsubscribed
	UnsubscribeConfirmed
)

type Mailer interface {
	SendVerification(ctx context.Context, email, verifyURL string) error
}

type Options struct {
	Store         Store
	Mailer        Mailer
	Logger        *log.Logger
	Environment   string
	PublicBaseURL string
	LinkSecret    string
	Now           Clock
	TokenTTL      time.Duration
}

type server struct {
	store         Store
	mailer        Mailer
	logger        *log.Logger
	environment   string
	publicBaseURL string
	linkSecret    string
	now           Clock
	tokenTTL      time.Duration
	signupLimiter *RateLimiter
	verifyLimiter *RateLimiter
	unsubLimiter  *RateLimiter
}

func (s *server) siteHomeURL() string {
	base := strings.TrimSpace(s.publicBaseURL)
	if base == "" {
		return "/"
	}
	return strings.TrimRight(base, "/")
}

func New(options Options) http.Handler {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	logger := options.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	s := &server{
		store:         options.Store,
		mailer:        options.Mailer,
		logger:        logger,
		environment:   strings.ToLower(strings.TrimSpace(options.Environment)),
		publicBaseURL: strings.TrimSuffix(strings.TrimSpace(options.PublicBaseURL), "/"),
		linkSecret:    strings.TrimSpace(options.LinkSecret),
		now:           now,
		tokenTTL:      options.TokenTTL,
		signupLimiter: NewRateLimiter(defaultSignupPerMinute, time.Minute, now),
		verifyLimiter: NewRateLimiter(defaultVerifyPerMinute, time.Minute, now),
		unsubLimiter:  NewRateLimiter(defaultUnsubPerMinute, time.Minute, now),
	}

	if s.tokenTTL <= 0 {
		s.tokenTTL = defaultTokenTTL
	}
	if s.environment == "" {
		s.environment = "staging"
	}
	if s.publicBaseURL == "" {
		s.publicBaseURL = "https://example.invalid"
	}
	if s.store == nil {
		s.store = unavailableStore{}
	}
	if s.mailer == nil {
		s.mailer = NoopMailer{}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /newsletter/signup", s.handleSignup)
	mux.HandleFunc("GET /newsletter/verify", s.handleVerify)
	mux.HandleFunc("GET /newsletter/unsubscribe", s.handleUnsubscribe)
	mux.HandleFunc("/newsletter/", s.handleNewsletterNotFound)
	mux.HandleFunc("/newsletter", s.handleNewsletterNotFound)
	mux.HandleFunc("/", s.handleNotFound)
	return mux
}

func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if !s.signupLimiter.Allow("signup:" + clientIP(r)) {
		writeJSONError(w, http.StatusTooManyRequests, "too many requests")
		return
	}

	var req signupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	email, err := normalizeEmail(req.Email)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid email")
		return
	}

	now := s.now().UTC()
	token, tokenHash, err := generateTokenPair()
	if err != nil {
		s.logger.Printf("signup token generation failed: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	needsVerification, err := s.store.Signup(r.Context(), email, tokenHash, now.Add(s.tokenTTL), now)
	if err != nil {
		s.logger.Printf("signup persistence failed: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if needsVerification {
		verifyURL := fmt.Sprintf("%s/newsletter/verify?token=%s", s.publicBaseURL, token)
		if err := s.mailer.SendVerification(r.Context(), email, verifyURL); err != nil {
			// Preserve anti-enumeration behavior while still surfacing operator signals.
			s.logger.Printf("verification mail send failed: %v", err)
		}
	}

	writeJSON(w, http.StatusAccepted, signupResponse{
		Status:  "accepted",
		Message: "If that email can be subscribed, a confirmation email has been sent.",
	})
}

func (s *server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if !s.verifyLimiter.Allow("verify:" + clientIP(r)) {
		s.writeVerifyError(w, r, http.StatusTooManyRequests, "too many requests")
		return
	}

	rawToken := strings.TrimSpace(r.URL.Query().Get("token"))
	if rawToken == "" {
		s.writeVerifyError(w, r, http.StatusBadRequest, "missing token")
		return
	}
	tokenHash := hashToken(rawToken)

	result, err := s.store.Verify(r.Context(), tokenHash, s.now().UTC())
	if err != nil {
		if errors.Is(err, errInvalidToken) {
			s.writeVerifyError(w, r, http.StatusBadRequest, "invalid token")
			return
		}
		if errors.Is(err, errExpiredToken) {
			s.writeVerifyError(w, r, http.StatusGone, "expired token")
			return
		}
		s.logger.Printf("verify failed: %v", err)
		s.writeVerifyError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	switch result {
	case VerifyConfirmed:
		s.writeVerifySuccess(w, r, http.StatusOK, "confirmed")
	case VerifyAlreadyUsed:
		s.writeVerifySuccess(w, r, http.StatusOK, "already_confirmed")
	default:
		s.writeVerifyError(w, r, http.StatusBadRequest, "invalid token")
	}
}

func (s *server) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if !s.unsubLimiter.Allow("unsubscribe:" + clientIP(r)) {
		s.writeUnsubscribeError(w, r, http.StatusTooManyRequests, "too many requests")
		return
	}

	rawToken := strings.TrimSpace(r.URL.Query().Get("token"))
	if rawToken == "" {
		s.writeUnsubscribeError(w, r, http.StatusBadRequest, "missing token")
		return
	}
	subscriberID, err := links.ParseUnsubscribeToken(s.linkSecret, rawToken)
	if err != nil {
		s.writeUnsubscribeError(w, r, http.StatusBadRequest, "invalid token")
		return
	}

	result, err := s.store.Unsubscribe(r.Context(), subscriberID, s.now().UTC())
	if err != nil {
		if errors.Is(err, errInvalidUnsubscribeToken) {
			s.writeUnsubscribeError(w, r, http.StatusBadRequest, "invalid token")
			return
		}
		s.logger.Printf("unsubscribe failed: %v", err)
		s.writeUnsubscribeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	switch result {
	case UnsubscribeConfirmed:
		s.writeUnsubscribeSuccess(w, r, http.StatusOK, "unsubscribed")
	case UnsubscribeAlreadyUnsubscribed:
		s.writeUnsubscribeSuccess(w, r, http.StatusOK, "already_unsubscribed")
	default:
		s.writeUnsubscribeError(w, r, http.StatusBadRequest, "invalid token")
	}
}

func (s *server) handleNewsletterNotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSONError(w, http.StatusNotFound, "not found")
}

func (s *server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSONError(w, http.StatusNotFound, "not found")
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || strings.ContainsAny(email, " \t\r\n") {
		return "", errors.New("invalid email")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || strings.ToLower(addr.Address) != email {
		return "", errors.New("invalid email")
	}
	return email, nil
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return errors.New("invalid request body")
	}
	if dec.More() {
		return errors.New("invalid request body")
	}
	return nil
}

func generateTokenPair() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

func (s *server) writeVerifySuccess(w http.ResponseWriter, r *http.Request, status int, verifyStatus string) {
	if wantsHTML(r) {
		switch verifyStatus {
		case "confirmed":
			s.writeVerifyHTML(w, status, verifyHTMLPage{
				Title:   "Email confirmed",
				Message: "Your subscription is active and future updates will arrive in your inbox.",
				Tone:    "success",
			})
		case "already_confirmed":
			s.writeVerifyHTML(w, status, verifyHTMLPage{
				Title:   "Already confirmed",
				Message: "This link was already used and your subscription remains active.",
				Tone:    "info",
			})
		default:
			s.writeVerifyHTML(w, status, verifyHTMLPage{
				Title:   "Verification complete",
				Message: "Your subscription state has been updated.",
				Tone:    "info",
			})
		}
		return
	}
	writeJSON(w, status, verifyResponse{Status: verifyStatus})
}

func (s *server) writeVerifyError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if wantsHTML(r) {
		page := verifyHTMLPage{
			Title:   "Verification issue",
			Message: "We could not verify this request. Please submit your email again.",
			Tone:    "error",
		}
		switch status {
		case http.StatusBadRequest:
			if message == "missing token" {
				page.Title = "Missing token"
				page.Message = "The verification link is incomplete. Open the full link from your email."
			} else if message == "invalid token" {
				page.Title = "Invalid link"
				page.Message = "This verification link is not valid. Request a new confirmation email."
			}
		case http.StatusGone:
			page.Title = "Link expired"
			page.Message = "This verification link has expired. Please sign up again to receive a new one."
		case http.StatusTooManyRequests:
			page.Title = "Too many attempts"
			page.Message = "Please wait a minute and try again."
		case http.StatusInternalServerError:
			page.Title = "Temporary issue"
			page.Message = "Please try again shortly."
		}
		s.writeVerifyHTML(w, status, page)
		return
	}
	writeJSONError(w, status, message)
}

func (s *server) writeUnsubscribeSuccess(w http.ResponseWriter, r *http.Request, status int, unsubStatus string) {
	if wantsHTML(r) {
		switch unsubStatus {
		case "unsubscribed":
			s.writeVerifyHTML(w, status, verifyHTMLPage{
				Title:   "You are unsubscribed",
				Message: "You will not receive future campaign emails from this list.",
				Tone:    "success",
			})
		case "already_unsubscribed":
			s.writeVerifyHTML(w, status, verifyHTMLPage{
				Title:   "Already unsubscribed",
				Message: "This address is already opted out from future campaign emails.",
				Tone:    "info",
			})
		default:
			s.writeVerifyHTML(w, status, verifyHTMLPage{
				Title:   "Subscription updated",
				Message: "Your email preferences have been updated.",
				Tone:    "info",
			})
		}
		return
	}
	writeJSON(w, status, verifyResponse{Status: unsubStatus})
}

func (s *server) writeUnsubscribeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if wantsHTML(r) {
		page := verifyHTMLPage{
			Title:   "Unsubscribe issue",
			Message: "We could not process this unsubscribe request.",
			Tone:    "error",
		}
		switch status {
		case http.StatusBadRequest:
			if message == "missing token" {
				page.Title = "Missing token"
				page.Message = "The unsubscribe link is incomplete. Open the full link from your email."
			} else if message == "invalid token" {
				page.Title = "Invalid link"
				page.Message = "This unsubscribe link is not valid."
			}
		case http.StatusTooManyRequests:
			page.Title = "Too many attempts"
			page.Message = "Please wait a minute and try again."
		case http.StatusInternalServerError:
			page.Title = "Temporary issue"
			page.Message = "Please try again shortly."
		}
		s.writeVerifyHTML(w, status, page)
		return
	}
	writeJSONError(w, status, message)
}

type verifyHTMLPage struct {
	Title   string
	Message string
	Tone    string
}

func (s *server) writeVerifyHTML(w http.ResponseWriter, status int, page verifyHTMLPage) {
	toneClass := "tone-info"
	if page.Tone == "success" {
		toneClass = "tone-success"
	} else if page.Tone == "error" {
		toneClass = "tone-error"
	}

	html := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>%s - Newsletter</title>
  <style>
    :root {
      --bg: #0b0e14;
      --panel: #101722;
      --text: #e0e5e9;
      --muted: #93a0aa;
      --success: #63d7b7;
      --info: #9fc4d0;
      --error: #ff9a8f;
      --line: rgba(255,255,255,0.12);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 20px;
      background:
        radial-gradient(circle at 20%% 20%%, rgba(109,158,163,0.18), transparent 40%%),
        radial-gradient(circle at 80%% 85%%, rgba(61,94,120,0.20), transparent 45%%),
        var(--bg);
      color: var(--text);
      font: 16px/1.5 Inter, system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
    }
    .card {
      width: min(560px, 100%%);
      background: linear-gradient(180deg, rgba(255,255,255,0.03), rgba(255,255,255,0.01));
      border: 1px solid var(--line);
      border-radius: 18px;
      padding: 28px;
      backdrop-filter: blur(10px);
      box-shadow: 0 20px 60px rgba(0,0,0,0.35);
    }
    .eyebrow {
      font-size: 12px;
      letter-spacing: .12em;
      text-transform: uppercase;
      color: var(--muted);
      margin: 0 0 10px;
    }
    h1 {
      margin: 0 0 10px;
      font-size: clamp(1.5rem, 4vw, 2rem);
      line-height: 1.15;
      letter-spacing: -0.02em;
    }
    p {
      margin: 0;
      color: var(--muted);
    }
    .status {
      margin: 18px 0 0;
      display: inline-flex;
      align-items: center;
      gap: 8px;
      border-radius: 999px;
      padding: 7px 12px;
      font-size: 12px;
      font-weight: 600;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      border: 1px solid var(--line);
    }
    .tone-success { color: var(--success); background: rgba(99,215,183,0.08); }
    .tone-info { color: var(--info); background: rgba(159,196,208,0.08); }
    .tone-error { color: var(--error); background: rgba(255,154,143,0.10); }
    .actions { margin-top: 22px; }
    a.button {
      display: inline-block;
      color: #0b0e14;
      text-decoration: none;
      font-weight: 700;
      background: #e0e5e9;
      border-radius: 999px;
      padding: 10px 16px;
      font-size: 14px;
    }
  </style>
</head>
<body>
  <main class="card" role="main">
    <p class="eyebrow">Newsletter</p>
    <h1>%s</h1>
    <p>%s</p>
    <p class="status %s">%s</p>
    <div class="actions">
      <a class="button" href="%s">Back to site</a>
    </div>
  </main>
</body>
</html>`,
		htmlEscape(page.Title), htmlEscape(page.Title), htmlEscape(page.Message), toneClass, htmlEscape(strings.ReplaceAll(page.Tone, "_", " ")), htmlEscape(s.siteHomeURL()))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, html)
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func wantsHTML(r *http.Request) bool {
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	if accept == "" {
		return false
	}
	return strings.Contains(accept, "text/html")
}

func clientIP(r *http.Request) string {
	remoteHost, ok := parseRemoteIP(r.RemoteAddr)
	if !ok {
		return "unknown"
	}
	if !isTrustedProxy(remoteHost) {
		return remoteHost
	}

	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff == "" {
		return remoteHost
	}

	// Trust only the right-most valid IP when request comes from a trusted local proxy.
	if forwarded, ok := rightMostValidForwardedIP(xff); ok {
		return forwarded
	}
	return remoteHost
}

func parseRemoteIP(remoteAddr string) (string, bool) {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		return "", false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", false
	}
	return ip.String(), true
}

func isTrustedProxy(ipRaw string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipRaw))
	return ip != nil && ip.IsLoopback()
}

func rightMostValidForwardedIP(xff string) (string, bool) {
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		ip := net.ParseIP(candidate)
		if ip != nil {
			return ip.String(), true
		}
	}
	return "", false
}

type unavailableStore struct{}

func (unavailableStore) Signup(context.Context, string, []byte, time.Time, time.Time) (bool, error) {
	return false, errors.New("store unavailable")
}

func (unavailableStore) Verify(context.Context, []byte, time.Time) (VerifyResult, error) {
	return VerifyInvalid, errors.New("store unavailable")
}

func (unavailableStore) Unsubscribe(context.Context, int64, time.Time) (UnsubscribeResult, error) {
	return UnsubscribeInvalid, errors.New("store unavailable")
}
