package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ipBucket struct {
	tokens  int
	resetAt time.Time
	mu      sync.Mutex
}

// RateLimiter is a per-IP token-bucket rate limiter with no external dependencies.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*ipBucket
	limit   int
	window  time.Duration
}

// NewRateLimiter returns a RateLimiter allowing requestsPerWindow requests per IP per window.
func NewRateLimiter(requestsPerWindow int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*ipBucket),
		limit:   requestsPerWindow,
		window:  window,
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &ipBucket{}
		rl.buckets[ip] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.After(b.resetAt) {
		b.tokens = rl.limit
		b.resetAt = now.Add(rl.window)
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

// Middleware wraps a handler and rejects requests that exceed the rate limit.
// Loopback addresses (127.x, ::1) are never rate-limited.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !isLoopback(ip) && !rl.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many requests"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopback(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

// clientIP extracts the client IP from X-Forwarded-For (set by Fly.io proxy) or RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
