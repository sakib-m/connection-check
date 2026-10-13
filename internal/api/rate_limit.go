package api

import (
	"sync"
	"time"
)

// RateLimiter manages hourly speed test quotas per IP address.
type RateLimiter struct {
	mu          sync.Mutex
	limits      map[string][]time.Time
	maxPerHour  int
	windowHours time.Duration
}

func NewRateLimiter(maxPerHour int) *RateLimiter {
	rl := &RateLimiter{
		limits:      make(map[string][]time.Time),
		maxPerHour:  maxPerHour,
		windowHours: 1 * time.Hour,
	}

	// Periodic garbage collection for stale IPs
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.windowHours)
	for ip, timestamps := range rl.limits {
		var valid []time.Time
		for _, t := range timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.limits, ip)
		} else {
			rl.limits[ip] = valid
		}
	}
}

// CheckLimit checks if IP is allowed without consuming quota.
func (rl *RateLimiter) CheckLimit(ip string) (allowed bool, remaining int, resetIn time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.windowHours)

	var valid []time.Time
	for _, t := range rl.limits[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.limits[ip] = valid

	count := len(valid)
	if count >= rl.maxPerHour {
		oldest := valid[0]
		resetIn = oldest.Add(rl.windowHours).Sub(now)
		if resetIn < 0 {
			resetIn = 0
		}
		return false, 0, resetIn
	}

	remaining = rl.maxPerHour - count
	return true, remaining, 0
}

// RecordTest records a consumed test attempt for the IP.
func (rl *RateLimiter) RecordTest(ip string) (allowed bool, remaining int, resetIn time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.windowHours)

	var valid []time.Time
	for _, t := range rl.limits[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.maxPerHour {
		oldest := valid[0]
		resetIn = oldest.Add(rl.windowHours).Sub(now)
		if resetIn < 0 {
			resetIn = 0
		}
		rl.limits[ip] = valid
		return false, 0, resetIn
	}

	valid = append(valid, now)
	rl.limits[ip] = valid
	remaining = rl.maxPerHour - len(valid)
	return true, remaining, 0
}
