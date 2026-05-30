package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// rateLimiter is a tiny per-key fixed-window counter. It is intentionally
// simple (no external deps): each key gets `limit` requests per 60s window.
type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*window
}

type window struct {
	count int
	reset time.Time
}

func newRateLimiter(rpm int) *rateLimiter {
	return &rateLimiter{limit: rpm, window: time.Minute, hits: make(map[string]*window)}
}

// allow reports whether a request for key is permitted and, if not, how long
// until the window resets.
func (rl *rateLimiter) allow(key string) (bool, time.Duration) {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	w := rl.hits[key]
	if w == nil || now.After(w.reset) {
		rl.hits[key] = &window{count: 1, reset: now.Add(rl.window)}
		// Opportunistically evict stale entries to bound memory.
		if len(rl.hits) > 4096 {
			for k, v := range rl.hits {
				if now.After(v.reset) {
					delete(rl.hits, k)
				}
			}
		}
		return true, 0
	}
	if w.count >= rl.limit {
		return false, time.Until(w.reset)
	}
	w.count++
	return true, 0
}

// rateLimited reports whether a path should be rate limited: state-changing
// methods on /v1, plus auth endpoints (to slow brute force). Read-only probes
// and asset fetches are never limited.
func rateLimited(r *http.Request) bool {
	if !pathHasPrefix(r.URL.Path, "/v1/") {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return true
	}
	return false
}

func pathHasPrefix(p, prefix string) bool {
	return len(p) >= len(prefix) && p[:len(prefix)] == prefix
}

// withRateLimit wraps next with per-IP rate limiting on write/auth endpoints.
func (s *Server) withRateLimit(next http.Handler) http.Handler {
	if s.cfg.RateLimitRPM <= 0 || s.limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rateLimited(r) {
			if ok, retry := s.limiter.allow(clientIP(r)); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded — slow down and retry")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
