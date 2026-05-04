package httpapi

import (
	"testing"
	"time"
)

func TestRateLimiterAllowsWithinLimitAndRejectsExcess(t *testing.T) {
	limiter := newRateLimiter(2, time.Minute)
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	if !limiter.allow("login:127.0.0.1", now) {
		t.Fatal("expected first attempt to be allowed")
	}
	if !limiter.allow("login:127.0.0.1", now.Add(time.Second)) {
		t.Fatal("expected second attempt to be allowed")
	}
	if limiter.allow("login:127.0.0.1", now.Add(2*time.Second)) {
		t.Fatal("expected third attempt inside the window to be rejected")
	}
}

func TestRateLimiterAllowsAfterWindow(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	if !limiter.allow("register:127.0.0.1", now) {
		t.Fatal("expected first attempt to be allowed")
	}
	if !limiter.allow("register:127.0.0.1", now.Add(2*time.Minute)) {
		t.Fatal("expected attempt after the window to be allowed")
	}
}
