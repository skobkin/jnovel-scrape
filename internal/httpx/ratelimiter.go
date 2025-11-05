package httpx

import (
	"context"
	"sync"
	"time"
)

// RateLimiter ensures a minimum interval between requests across goroutines.
type RateLimiter struct {
	interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

// NewRateLimiter creates a limiter with the provided interval.
func NewRateLimiter(interval time.Duration) *RateLimiter {
	if interval <= 0 {
		interval = time.Millisecond
	}
	return &RateLimiter{interval: interval}
}

// Wait blocks until the caller is allowed to issue the next request.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		r.mu.Lock()
		now := time.Now()
		if r.next.IsZero() || now.After(r.next) || now.Equal(r.next) {
			r.next = now.Add(r.interval)
			r.mu.Unlock()
			return nil
		}
		wait := r.next.Sub(now)
		r.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
			// Loop to verify the slot.
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		}
	}
}
