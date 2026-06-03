package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type ipBucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is a per-IP token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*ipBucket
	rate     float64 // tokens added per second
	capacity float64 // maximum burst size
}

// NewRateLimiter creates a limiter that allows rate tokens/second with the given burst capacity.
// Apply chimiddleware.RealIP before this middleware so r.RemoteAddr holds the true client IP.
func NewRateLimiter(rate, capacity float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*ipBucket),
		rate:     rate,
		capacity: capacity,
	}
	go rl.runCleanup()
	return rl
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &ipBucket{tokens: rl.capacity, lastSeen: now}
		rl.buckets[ip] = b
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens = min(rl.capacity, b.tokens+elapsed*rl.rate)
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// runCleanup removes buckets that have been idle for 10+ minutes.
func (rl *RateLimiter) runCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for ip, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Limit returns middleware that rate-limits requests by client IP.
func (rl *RateLimiter) Limit() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if !rl.allow(ip) {
				writeErrJSON(w, http.StatusTooManyRequests, "RATE_LIMITED",
					"Too many requests. Please slow down and try again later.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
