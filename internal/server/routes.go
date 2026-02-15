package server

import (
	"net/http"
	"strings"
)

func registerAPIRoutes(mux *http.ServeMux, srv *Server) {
	mux.HandleFunc("/api/v1/websites/", srv.handleWebsiteAPI)
}

func (s *Server) handleWebsiteAPI(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/apply"):
		s.handleApply(w, r)
	case strings.HasSuffix(r.URL.Path, "/releases"):
		s.handleRelease(w, r)
	case strings.HasSuffix(r.URL.Path, "/logs"):
		s.handleLogs(w, r)
	default:
		http.NotFound(w, r)
	}
}

func actorFromRequest(r *http.Request) string {
	actor := strings.TrimSpace(r.Header.Get("X-Actor"))
	if actor == "" {
		return "local"
	}
	return actor
}
