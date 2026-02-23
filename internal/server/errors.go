package server

import "net/http"

func writeAPIError(w http.ResponseWriter, status int, message string, details []string) {
	resp := map[string]any{"error": message}
	if len(details) > 0 {
		resp["details"] = details
	}
	writeJSON(w, status, resp)
}

func (s *Server) writeInternalAPIError(w http.ResponseWriter, r *http.Request, message string, err error, attrs ...any) {
	logAttrs := make([]any, 0, len(attrs)+2)
	logAttrs = append(logAttrs, "error", err)
	logAttrs = append(logAttrs, attrs...)
	s.logger.ErrorContext(r.Context(), message, logAttrs...)
	writeAPIError(w, http.StatusInternalServerError, message, nil)
}
