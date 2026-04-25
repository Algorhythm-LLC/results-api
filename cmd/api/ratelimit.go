package main

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/httprate"
)

// newRateLimitMiddlewareFromEnv returns middleware that rate-limits by real client
// IP and URL path, or a no-op when RESULTS_API_RATE_LIMIT_REQUESTS is 0 or invalid.
// Defaults: 200 requests per window per (IP+path), window 1m. Uses KeyByRealIP (XFF / X-Real-IP) when behind a proxy; pair with chi middleware.RealIP.
func newRateLimitMiddlewareFromEnv() func(next http.Handler) http.Handler {
	reqPerWindow := 200
	if s := os.Getenv("RESULTS_API_RATE_LIMIT_REQUESTS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			reqPerWindow = n
		}
	}
	if reqPerWindow <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	window := time.Minute
	if s := os.Getenv("RESULTS_API_RATE_LIMIT_WINDOW"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			window = d
		}
	}
	slog.Info("results-api rate limit enabled", "requests", reqPerWindow, "window", window, "key", "real_ip+path")
	return httprate.Limit(
		reqPerWindow,
		window,
		httprate.WithKeyFuncs(httprate.KeyByRealIP, httprate.KeyByEndpoint),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}` + "\n"))
		}),
	)
}
