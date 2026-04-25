package main

import (
	"testing"
	"time"
)

func TestNewSummaryCacheFromEnv_disabled(t *testing.T) {
	t.Setenv("RESULTS_API_SUMMARY_CACHE_SIZE", "0")
	if c := newSummaryCacheFromEnv(); c != nil {
		t.Fatal("expected nil when size is 0")
	}
}

func TestSummaryCache_getPut(t *testing.T) {
	t.Setenv("RESULTS_API_SUMMARY_CACHE_SIZE", "10")
	t.Setenv("RESULTS_API_SUMMARY_CACHE_TTL", "1h")
	c := newSummaryCacheFromEnv()
	if c == nil {
		t.Fatal("expected cache")
	}
	runID := "run-1"
	s := runSummary{RunID: runID, PnLAbs: 42}
	c.put(runID, s)
	got, ok := c.get(runID)
	if !ok || got.PnLAbs != 42 {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestSummaryCache_expires(t *testing.T) {
	t.Setenv("RESULTS_API_SUMMARY_CACHE_SIZE", "10")
	t.Setenv("RESULTS_API_SUMMARY_CACHE_TTL", "30ms")
	c := newSummaryCacheFromEnv()
	if c == nil {
		t.Fatal("expected cache")
	}
	runID := "run-exp"
	c.put(runID, runSummary{RunID: runID})
	time.Sleep(60 * time.Millisecond)
	_, ok := c.get(runID)
	if ok {
		t.Fatal("expected miss after TTL")
	}
}

func TestNewSummaryCacheFromEnv_customSizeAndTTL(t *testing.T) {
	t.Setenv("RESULTS_API_SUMMARY_CACHE_SIZE", "32")
	t.Setenv("RESULTS_API_SUMMARY_CACHE_TTL", "2m15s")
	c := newSummaryCacheFromEnv()
	if c == nil {
		t.Fatal("expected cache")
	}
	if c.size != 32 || c.ttl != 2*time.Minute+15*time.Second {
		t.Fatalf("size=%d ttl=%v", c.size, c.ttl)
	}
}
