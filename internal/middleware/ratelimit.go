package middleware

import (
	"net/http"
	"sync"
	"time"
)

type visitor struct {
	tokens  int
	lastReq time.Time
}

type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	limit    int
	burst    int
	ttl      time.Duration
}

func NewRateLimiter(requestsPerSecond, burst int) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    requestsPerSecond,
		burst:    burst,
		ttl:      5 * time.Minute,
	}
}

func (rl *RateLimiter) getVisitor(ip string) *visitor {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{tokens: rl.burst, lastReq: time.Now()}
		rl.visitors[ip] = v
	}
	return v
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for ip, v := range rl.visitors {
		if now.Sub(v.lastReq) > rl.ttl {
			delete(rl.visitors, ip)
		}
	}
}

func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	go func() {
		ticker := time.NewTicker(rl.ttl)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			v := rl.getVisitor(ip)

			rl.mu.Lock()
			defer rl.mu.Unlock()

			now := time.Now()
			elapsed := now.Sub(v.lastReq)
			v.tokens += int(elapsed.Seconds() * float64(rl.limit))
			if v.tokens > rl.burst {
				v.tokens = rl.burst
			}
			v.lastReq = now

			if v.tokens < 1 {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"success":false,"error":{"code":"RATE_LIMITED","message":"Too many requests"}}`))
				return
			}

			v.tokens--
			next.ServeHTTP(w, r)
		})
	}
}
