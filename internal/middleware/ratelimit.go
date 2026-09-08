package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alrazihi/civora/internal/shared"
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
	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewRateLimiter(requestsPerSecond, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    requestsPerSecond,
		burst:    burst,
		ttl:      5 * time.Minute,
		stopCh:   make(chan struct{}),
	}
	go rl.runCleanup()
	return rl
}

func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stopCh)
	})
}

func (rl *RateLimiter) runCleanup() {
	ticker := time.NewTicker(rl.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanup()
		}
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
				body := []byte(`{"success":false,"error":{"code":"RATE_LIMITED","message":"Too many requests"}}`)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				shared.WriteBody(w, body)
				return
			}

			v.tokens--
			next.ServeHTTP(w, r)
		})
	}
}

type authAttempt struct {
	failures    int
	lastFail    time.Time
	lockedUntil time.Time
}

type UserRateLimiter struct {
	attempts        map[string]*authAttempt
	mu              sync.RWMutex
	maxFailures     int
	lockoutDuration time.Duration
	failureWindow   time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
}

func NewUserRateLimiter(maxFailures int, lockoutDuration, failureWindow time.Duration) *UserRateLimiter {
	rl := &UserRateLimiter{
		attempts:        make(map[string]*authAttempt),
		maxFailures:     maxFailures,
		lockoutDuration: lockoutDuration,
		failureWindow:   failureWindow,
		stopCh:          make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-rl.stopCh:
				return
			case <-ticker.C:
				rl.cleanup()
			}
		}
	}()

	return rl
}

func (rl *UserRateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stopCh)
	})
}

func (rl *UserRateLimiter) getAttempt(email string) *authAttempt {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	emailLower := strings.ToLower(strings.TrimSpace(email))
	a, exists := rl.attempts[emailLower]
	if !exists {
		a = &authAttempt{}
		rl.attempts[emailLower] = a
	}
	return a
}

func (rl *UserRateLimiter) RecordFailedAttempt(email string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	emailLower := strings.ToLower(strings.TrimSpace(email))
	now := time.Now()

	a, exists := rl.attempts[emailLower]
	if !exists {
		a = &authAttempt{}
		rl.attempts[emailLower] = a
	}

	if now.Sub(a.lastFail) > rl.failureWindow {
		a.failures = 0
	}

	a.failures++
	a.lastFail = now

	if a.failures >= rl.maxFailures {
		a.lockedUntil = now.Add(rl.lockoutDuration)
	}
}

func (rl *UserRateLimiter) IsLocked(email string) (bool, time.Duration) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	emailLower := strings.ToLower(strings.TrimSpace(email))
	a, exists := rl.attempts[emailLower]
	if !exists {
		return false, 0
	}

	now := time.Now()
	if a.lockedUntil.After(now) {
		return true, a.lockedUntil.Sub(now)
	}

	if a.failures >= rl.maxFailures && now.Sub(a.lastFail) <= rl.failureWindow {
		remaining := rl.failureWindow - now.Sub(a.lastFail)
		return true, remaining
	}

	return false, 0
}

func (rl *UserRateLimiter) RecordSuccessfulAttempt(email string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	emailLower := strings.ToLower(strings.TrimSpace(email))
	delete(rl.attempts, emailLower)
}

func (rl *UserRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for email, a := range rl.attempts {
		if now.Sub(a.lastFail) > rl.failureWindow && now.After(a.lockedUntil) {
			delete(rl.attempts, email)
		}
	}
}

func (rl *UserRateLimiter) CheckRateLimit(email string) (bool, int, time.Duration) {
	locked, remaining := rl.IsLocked(email)
	if locked {
		return false, http.StatusTooManyRequests, remaining
	}
	return true, 0, 0
}
