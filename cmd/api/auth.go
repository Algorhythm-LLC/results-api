package main

import (
	"net/http"
	"strings"
)

// parseAPIKeySet loads a deduplicated set of keys from RESULTS_API_API_KEYS
// (comma-separated). Empty or unset means auth is disabled.
func parseAPIKeySet(raw string) map[string]struct{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var parts []string
	for _, p := range strings.Split(raw, ",") {
		if k := strings.TrimSpace(p); k != "" {
			parts = append(parts, k)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(parts))
	for _, k := range parts {
		m[k] = struct{}{}
	}
	return m
}

// apiKeyMiddleware enforces X-API-Key when the key set is non-empty.
// /healthz and /readyz stay unauthenticated.
func apiKeyMiddleware(allowed map[string]struct{}) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}
			if len(allowed) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			k := r.Header.Get("X-API-Key")
			if k == "" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"missing X-API-Key"}`))
				return
			}
			if _, ok := allowed[k]; !ok {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid API key"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
