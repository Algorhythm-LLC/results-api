package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRateLimitMiddlewareFromEnv_disabled(t *testing.T) {
	t.Setenv("RESULTS_API_RATE_LIMIT_REQUESTS", "0")
	mw := newRateLimitMiddlewareFromEnv()
	calls := 0
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	for range 3 {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/r/summary", nil)
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 when limit disabled", rr.Code)
		}
	}
	if calls != 3 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestNewRateLimitMiddlewareFromEnv_429(t *testing.T) {
	t.Setenv("RESULTS_API_RATE_LIMIT_REQUESTS", "1")
	t.Setenv("RESULTS_API_RATE_LIMIT_WINDOW", "1h")
	mw := newRateLimitMiddlewareFromEnv()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	p := "/api/v1/runs/one/summary"
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, httptest.NewRequest(http.MethodGet, p, nil))
	if rr1.Code != http.StatusOK {
		t.Fatalf("first: %d", rr1.Code)
	}
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, p, nil))
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second: want 429, got %d", rr2.Code)
	}
	// other path: separate bucket (KeyByEndpoint)
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/v1/compare/runs", nil))
	if rr3.Code != http.StatusOK {
		t.Fatalf("other path: %d", rr3.Code)
	}
}
