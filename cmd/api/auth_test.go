package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyMiddleware_disabled(t *testing.T) {
	var hit bool
	h := apiKeyMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/x/summary", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if !hit || rr.Code != http.StatusNoContent {
		t.Fatalf("want pass-through, code=%d hit=%v", rr.Code, hit)
	}
}

func TestAPIKeyMiddleware_required(t *testing.T) {
	allowed := map[string]struct{}{"secret": {}}
	h := apiKeyMiddleware(allowed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/x/summary", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without key, got %d", rr.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/runs/x/summary", nil)
	req2.Header.Set("X-API-Key", "secret")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("want 200 with key, got %d", rr2.Code)
	}

	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("healthz bypass: got %d", rr3.Code)
	}
}

func TestParseAPIKeySet(t *testing.T) {
	if parseAPIKeySet("") != nil {
		t.Fatal("empty should be nil")
	}
	if parseAPIKeySet("  , , ") != nil {
		t.Fatal("only commas should be nil")
	}
	m := parseAPIKeySet(" a, b ,b,")
	if len(m) != 2 {
		t.Fatalf("want 2 keys, got %d", len(m))
	}
}
