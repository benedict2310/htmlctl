package server

import (
	"encoding/json"
	"net/http"
)

type apiError struct {
	Error string `json:"error"`
}

func New() http.Handler {
	mux := http.NewServeMux()
	newsletterHandler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSONError(w, http.StatusNotImplemented, "newsletter endpoint not yet enabled")
	}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	mux.HandleFunc("/newsletter", newsletterHandler)
	mux.HandleFunc("/newsletter/", newsletterHandler)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONError(w, http.StatusNotFound, "not found")
	})

	return mux
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError{Error: message})
}
