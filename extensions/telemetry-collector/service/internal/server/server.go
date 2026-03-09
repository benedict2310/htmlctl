package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	stdpath "path"
	"regexp"
	"strings"
	"time"
)

const (
	defaultRequestsPerMinute = 120
	maxPathBytes             = 1024
	maxAttrs                 = 16
	maxAttrKeyBytes          = 64
	maxAttrValueBytes        = 256
	maxOccurredFutureSkew    = 24 * time.Hour
	maxOccurredPastSkew      = 30 * 24 * time.Hour
)

var (
	eventNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
	sessionIDRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
	attrsKeyRE  = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_-]*$`)
)

type Forwarder interface {
	Forward(context.Context, ForwardRequest) (*ForwardResponse, error)
}

type ForwardRequest struct {
	Host        string
	Scheme      string
	ContentType string
	Body        []byte
}

type ForwardResponse struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

type Options struct {
	Logger         *log.Logger
	PublicBaseURL  string
	AllowedEvents  []string
	MaxBodyBytes   int
	MaxEvents      int
	Now            Clock
	Forwarder      Forwarder
	RequestsPerMin int
}

type server struct {
	logger        *log.Logger
	now           Clock
	forwarder     Forwarder
	origin        originConfig
	allowedEvents map[string]struct{}
	maxBodyBytes  int64
	maxEvents     int
	limiter       *RateLimiter
}

type originConfig struct {
	Scheme string
	Host   string
	Port   string
	Base   string
}

type ingestRequest struct {
	Events []ingestEvent `json:"events"`
}

type ingestEvent struct {
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	OccurredAt string            `json:"occurredAt,omitempty"`
	SessionID  string            `json:"sessionId,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`
}

type apiError struct {
	Error string `json:"error"`
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
	forwarder := options.Forwarder
	if forwarder == nil {
		forwarder = unavailableForwarder{}
	}
	origin := mustOriginConfig(options.PublicBaseURL)
	allowed := make(map[string]struct{}, len(options.AllowedEvents))
	for _, eventName := range options.AllowedEvents {
		allowed[strings.TrimSpace(eventName)] = struct{}{}
	}
	if len(allowed) == 0 {
		allowed["page_view"] = struct{}{}
		allowed["link_click"] = struct{}{}
		allowed["cta_click"] = struct{}{}
		allowed["newsletter_signup"] = struct{}{}
	}
	if options.MaxBodyBytes <= 0 {
		options.MaxBodyBytes = 32 << 10
	}
	if options.MaxEvents <= 0 {
		options.MaxEvents = 10
	}
	if options.RequestsPerMin <= 0 {
		options.RequestsPerMin = defaultRequestsPerMinute
	}
	s := &server{
		logger:        logger,
		now:           now,
		forwarder:     forwarder,
		origin:        origin,
		allowedEvents: allowed,
		maxBodyBytes:  int64(options.MaxBodyBytes),
		maxEvents:     options.MaxEvents,
		limiter:       NewRateLimiter(options.RequestsPerMin, time.Minute, now),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /site-telemetry/v1/events", s.handleIngest)
	mux.HandleFunc("/site-telemetry/", s.handleNotFound)
	mux.HandleFunc("/site-telemetry", s.handleNotFound)
	mux.HandleFunc("/", s.handleNotFound)
	return mux
}

func mustOriginConfig(raw string) originConfig {
	cfg, err := parseOriginConfig(raw)
	if err != nil {
		panic(err)
	}
	return cfg
}

func parseOriginConfig(raw string) (originConfig, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return originConfig{}, err
	}
	host, err := normalizeHost(parsed.Host)
	if err != nil {
		return originConfig{}, err
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	port := normalizedPort(parsed.Host, scheme)
	return originConfig{Scheme: scheme, Host: host, Port: port, Base: strings.TrimRight(parsed.String(), "/")}, nil
}

func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

func (s *server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.Allow("ingest:" + clientIP(r)) {
		writeJSONError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	if err := validateContentType(r.Header.Get("Content-Type")); err != nil {
		writeJSONError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}
	reqScheme, err := requestScheme(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request scheme")
		return
	}
	reqHost, err := normalizeHost(r.Host)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid host")
		return
	}
	if reqHost != s.origin.Host || normalizedPort(r.Host, reqScheme) != s.origin.Port || reqScheme != s.origin.Scheme {
		writeJSONError(w, http.StatusBadRequest, "request origin does not match configured public base URL")
		return
	}
	if err := validateOriginHeader(r.Header.Get("Origin"), s.origin, reqScheme, r.Host); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		if isMaxBytesError(err) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var req ingestRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeJSONError(w, http.StatusBadRequest, "request body must contain a single JSON object")
		return
	}
	if len(req.Events) == 0 {
		writeJSONError(w, http.StatusBadRequest, "events must contain at least one event")
		return
	}
	if len(req.Events) > s.maxEvents {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("events must contain at most %d items", s.maxEvents))
		return
	}
	for i, event := range req.Events {
		if err := s.validateEvent(event, s.now().UTC()); err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("events[%d]: %v", i, err))
			return
		}
	}

	resp, err := s.forwarder.Forward(r.Context(), ForwardRequest{
		Host:        s.origin.Host,
		Scheme:      s.origin.Scheme,
		ContentType: normalizeForwardContentType(r.Header.Get("Content-Type")),
		Body:        body,
	})
	if err != nil {
		s.logger.Printf("telemetry forward failed: %v", err)
		writeJSONError(w, http.StatusBadGateway, "telemetry ingest unavailable")
		return
	}
	if resp.StatusCode >= 500 {
		s.logger.Printf("telemetry forward upstream failure: status=%d body=%s", resp.StatusCode, truncateForLog(resp.Body))
		writeJSONError(w, http.StatusBadGateway, "telemetry ingest unavailable")
		return
	}
	if resp.StatusCode >= 400 {
		if resp.ContentType != "" {
			w.Header().Set("Content-Type", resp.ContentType)
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(resp.Body)
		return
	}
	if resp.ContentType != "" {
		w.Header().Set("Content-Type", resp.ContentType)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

func (s *server) validateEvent(event ingestEvent, now time.Time) error {
	name := strings.TrimSpace(event.Name)
	if _, ok := s.allowedEvents[name]; !ok {
		return fmt.Errorf("event %q is not allowed", name)
	}
	if !eventNameRE.MatchString(name) {
		return fmt.Errorf("event name must match [a-zA-Z0-9][a-zA-Z0-9_-]* and be at most 64 characters")
	}
	if _, err := normalizePath(event.Path); err != nil {
		return err
	}
	if _, err := normalizeOccurredAt(event.OccurredAt, now); err != nil {
		return err
	}
	if _, err := normalizeSessionID(event.SessionID); err != nil {
		return err
	}
	if err := validateAttrs(event.Attrs); err != nil {
		return err
	}
	return nil
}

func normalizePath(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("path must not contain null bytes")
	}
	unescaped, err := url.PathUnescape(raw)
	if err != nil {
		return "", fmt.Errorf("path must be a valid URL path")
	}
	if strings.ContainsRune(unescaped, 0) {
		return "", fmt.Errorf("path must not contain null bytes")
	}
	clean := stdpath.Clean(unescaped)
	if clean == "." {
		clean = "/"
	}
	if !strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("path must start with /")
	}
	if len([]byte(clean)) > maxPathBytes {
		return "", fmt.Errorf("path must be at most %d bytes", maxPathBytes)
	}
	return clean, nil
}

func normalizeOccurredAt(value string, now time.Time) (*string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, nil
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("invalid occurredAt: must be ISO 8601 / RFC 3339")
	}
	ts = ts.UTC()
	if ts.After(now.Add(maxOccurredFutureSkew)) {
		return nil, fmt.Errorf("occurredAt is too far in the future")
	}
	if ts.Before(now.Add(-maxOccurredPastSkew)) {
		return nil, fmt.Errorf("occurredAt is too far in the past")
	}
	formatted := ts.Format(time.RFC3339)
	return &formatted, nil
}

func normalizeSessionID(value string) (*string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, nil
	}
	if !sessionIDRE.MatchString(raw) {
		return nil, fmt.Errorf("sessionId must match ^[a-zA-Z0-9_-]{1,128}$")
	}
	return &raw, nil
}

func validateAttrs(attrs map[string]string) error {
	if len(attrs) > maxAttrs {
		return fmt.Errorf("attrs must contain at most %d keys", maxAttrs)
	}
	for key, value := range attrs {
		if len([]byte(key)) > maxAttrKeyBytes {
			return fmt.Errorf("attrs key %q exceeds %d bytes", key, maxAttrKeyBytes)
		}
		if !attrsKeyRE.MatchString(key) {
			return fmt.Errorf("attrs key %q must match ^[a-zA-Z0-9_][a-zA-Z0-9_-]*$", key)
		}
		if len([]byte(value)) > maxAttrValueBytes {
			return fmt.Errorf("attrs value for %q exceeds %d bytes", key, maxAttrValueBytes)
		}
		if strings.ContainsRune(value, 0) {
			return fmt.Errorf("attrs value for %q must not contain null bytes", key)
		}
	}
	return nil
}

func validateContentType(raw string) error {
	contentType := strings.TrimSpace(raw)
	if contentType == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("invalid Content-Type header")
	}
	switch mediaType {
	case "application/json", "text/plain":
		return nil
	default:
		return fmt.Errorf("unsupported Content-Type %q; expected application/json or text/plain", mediaType)
	}
}

func normalizeForwardContentType(raw string) string {
	contentType := strings.TrimSpace(raw)
	if contentType == "" {
		return "application/json"
	}
	return contentType
}

func validateOriginHeader(rawOrigin string, configured originConfig, reqScheme, rawHost string) error {
	origin := strings.TrimSpace(rawOrigin)
	if origin == "" {
		return nil
	}
	originURL, err := url.Parse(origin)
	if err != nil || strings.TrimSpace(originURL.Host) == "" {
		return fmt.Errorf("invalid Origin header")
	}
	originScheme := strings.ToLower(strings.TrimSpace(originURL.Scheme))
	if originScheme != configured.Scheme {
		return fmt.Errorf("cross-origin telemetry ingest is not supported")
	}
	originHost, err := normalizeHost(originURL.Host)
	if err != nil {
		return fmt.Errorf("invalid Origin header")
	}
	requestHost, err := normalizeHost(rawHost)
	if err != nil {
		return fmt.Errorf("invalid host")
	}
	originPort := normalizedPort(originURL.Host, originScheme)
	requestPort := normalizedPort(rawHost, reqScheme)
	if originHost != configured.Host || originHost != requestHost || originPort != configured.Port || originPort != requestPort {
		return fmt.Errorf("cross-origin telemetry ingest is not supported")
	}
	return nil
}

func requestScheme(r *http.Request) (string, error) {
	if r == nil {
		return "", errors.New("request is required")
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		scheme := strings.ToLower(strings.TrimSpace(first))
		if scheme == "http" || scheme == "https" {
			return scheme, nil
		}
		return "", errors.New("unsupported scheme")
	}
	if r.TLS != nil {
		return "https", nil
	}
	return "http", nil
}

func normalizeHost(rawHost string) (string, error) {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return "", errors.New("host is required")
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return "", errors.New("host is required")
	}
	return host, nil
}

func normalizedPort(rawHost, scheme string) string {
	port := ""
	if strings.Contains(rawHost, ":") {
		if parsed, err := url.Parse("http://" + rawHost); err == nil {
			port = strings.TrimSpace(parsed.Port())
		}
	}
	if port != "" {
		return port
	}
	if scheme == "https" {
		return "443"
	}
	return "80"
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSONError(w, http.StatusNotFound, "not found")
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

func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func truncateForLog(body []byte) string {
	const max = 256
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "..."
}

type HTTPForwarder struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func (f HTTPForwarder) Forward(ctx context.Context, req ForwardRequest) (*ForwardResponse, error) {
	if strings.TrimSpace(f.BaseURL) == "" {
		return nil, errors.New("base URL is required")
	}
	if strings.TrimSpace(f.Token) == "" {
		return nil, errors.New("token is required")
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	upstreamURL := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/") + "/collect/v1/events"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+f.Token)
	httpReq.Header.Set("Content-Type", req.ContentType)
	httpReq.Header.Set("X-Forwarded-Proto", req.Scheme)
	httpReq.Host = req.Host

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return &ForwardResponse{StatusCode: resp.StatusCode, ContentType: strings.TrimSpace(resp.Header.Get("Content-Type")), Body: body}, nil
}

type unavailableForwarder struct{}

func (unavailableForwarder) Forward(context.Context, ForwardRequest) (*ForwardResponse, error) {
	return nil, errors.New("forwarder unavailable")
}
