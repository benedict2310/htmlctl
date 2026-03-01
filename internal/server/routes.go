package server

import (
	"net/http"
	"strings"
)

func registerAPIRoutes(mux *http.ServeMux, srv *Server) {
	mux.HandleFunc("/api/v1/websites", srv.handleWebsites)
	mux.HandleFunc("/api/v1/websites/", srv.handleWebsiteAPI)
	mux.HandleFunc("/api/v1/domains", srv.handleDomains)
	mux.HandleFunc("/api/v1/domains/", srv.handleDomains)
}

func (s *Server) handleWebsiteAPI(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/apply"):
		s.handleApply(w, r)
	case strings.HasSuffix(r.URL.Path, "/promote"):
		s.handlePromote(w, r)
	case strings.HasSuffix(r.URL.Path, "/releases"):
		s.handleRelease(w, r)
	case strings.HasSuffix(r.URL.Path, "/rollback"):
		s.handleRollback(w, r)
	case strings.HasSuffix(r.URL.Path, "/manifest"):
		s.handleManifest(w, r)
	case strings.HasSuffix(r.URL.Path, "/logs"):
		s.handleLogs(w, r)
	case strings.HasSuffix(r.URL.Path, "/backends"):
		s.handleBackends(w, r)
	case strings.HasSuffix(r.URL.Path, "/status"):
		s.handleStatus(w, r)
	case strings.HasSuffix(r.URL.Path, "/telemetry/events"):
		s.handleTelemetryEvents(w, r)
	case strings.HasSuffix(r.URL.Path, "/environments"):
		s.handleEnvironments(w, r)
	default:
		http.NotFound(w, r)
	}
}

func actorFromRequest(r *http.Request) string {
	if state, ok := authStateFromContext(r.Context()); !ok || !state.actorTrusted {
		return "local"
	}
	actor := strings.TrimSpace(r.Header.Get("X-Actor"))
	if actor == "" {
		return "local"
	}
	return actor
}
