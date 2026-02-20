package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

type authStateKey struct{}

type authState struct {
	actorTrusted bool
}

func withAuthState(ctx context.Context, state authState) context.Context {
	return context.WithValue(ctx, authStateKey{}, state)
}

func authStateFromContext(ctx context.Context) (authState, bool) {
	if ctx == nil {
		return authState{}, false
	}
	state, ok := ctx.Value(authStateKey{}).(authState)
	return state, ok
}

func authMiddleware(expectedToken string, logger *slog.Logger) func(http.Handler) http.Handler {
	expectedToken = strings.TrimSpace(expectedToken)
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		if expectedToken == "" {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Rollout-compatibility mode: when no token is configured, API auth is
				// intentionally bypassed and caller-supplied actor headers are trusted.
				next.ServeHTTP(w, r.WithContext(withAuthState(r.Context(), authState{actorTrusted: true})))
			})
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			presentedToken, ok := parseBearerToken(r.Header.Get("Authorization"))
			if !ok || !constantTimeTokenEqual(expectedToken, presentedToken) {
				logger.Warn("unauthorized API request rejected",
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				writeAPIError(w, http.StatusUnauthorized, "unauthorized", nil)
				return
			}

			next.ServeHTTP(w, r.WithContext(withAuthState(r.Context(), authState{actorTrusted: true})))
		})
	}
}

func parseBearerToken(headerValue string) (string, bool) {
	parts := strings.Fields(strings.TrimSpace(headerValue))
	if len(parts) != 2 {
		return "", false
	}
	// RFC 7235 treats auth-scheme tokens as case-insensitive.
	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}

func constantTimeTokenEqual(expected, presented string) bool {
	expectedDigest := sha256.Sum256([]byte(expected))
	presentedDigest := sha256.Sum256([]byte(presented))
	return subtle.ConstantTimeCompare(expectedDigest[:], presentedDigest[:]) == 1
}
