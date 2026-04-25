package main

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2"
)

// summaryCache is an LRU of run_id → run summary with a fixed TTL on insert.
// Nil means disabled (RESULTS_API_SUMMARY_CACHE_SIZE ≤ 0).
type summaryCache struct {
	lru  *lru.Cache[string, summaryCacheEntry]
	ttl  time.Duration
	size int // max entries
	mu   sync.Mutex
}

type summaryCacheEntry struct {
	s   runSummary
	exp time.Time
}

func newSummaryCacheFromEnv() *summaryCache {
	size := 256
	if raw := os.Getenv("RESULTS_API_SUMMARY_CACHE_SIZE"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			size = n
		}
	}
	if size <= 0 {
		return nil
	}
	ttl := 60 * time.Second
	if s := os.Getenv("RESULTS_API_SUMMARY_CACHE_TTL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			ttl = d
		}
	}
	inner, err := lru.New[string, summaryCacheEntry](size)
	if err != nil {
		return nil
	}
	return &summaryCache{lru: inner, ttl: ttl, size: size}
}

func (c *summaryCache) get(runID string) (runSummary, bool) {
	if c == nil {
		return runSummary{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.lru.Get(runID)
	if !ok {
		return runSummary{}, false
	}
	if time.Now().After(v.exp) {
		_ = c.lru.Remove(runID)
		return runSummary{}, false
	}
	return v.s, true
}

func (c *summaryCache) put(runID string, s runSummary) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.Add(runID, summaryCacheEntry{
		s:   s,
		exp: time.Now().Add(c.ttl),
	})
}
