package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	attempts    map[string][]time.Time
	lastCleanup time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:    limit,
		window:   window,
		attempts: make(map[string][]time.Time),
	}
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) >= l.window {
		for existingKey, attempts := range l.attempts {
			if len(attempts) == 0 || !attempts[len(attempts)-1].After(cutoff) {
				delete(l.attempts, existingKey)
			}
		}
		l.lastCleanup = now
	}
	recent := l.attempts[key][:0]
	for _, attempt := range l.attempts[key] {
		if attempt.After(cutoff) {
			recent = append(recent, attempt)
		}
	}
	if len(recent) >= l.limit {
		l.attempts[key] = recent
		return false
	}

	l.attempts[key] = append(recent, now)
	return true
}

func (l *rateLimiter) middleware(action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := action + ":" + clientIP(r)
		if !l.allow(key, time.Now().UTC()) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"detail": "Too many requests"})
			return
		}
		next(w, r)
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
