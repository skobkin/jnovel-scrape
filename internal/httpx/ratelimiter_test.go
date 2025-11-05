package httpx

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterWaitAllowsProgress(t *testing.T) {
	limiter := NewRateLimiter(10 * time.Millisecond)

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("first wait error: %v", err)
	}

	start := time.Now()
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("second wait error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 8*time.Millisecond {
		t.Fatalf("expected limiter to enforce delay, got %s", elapsed)
	}
}

func TestRateLimiterWaitCanceled(t *testing.T) {
	limiter := NewRateLimiter(50 * time.Millisecond)

	// Prime the limiter so the next call needs to wait.
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("prime wait error: %v", err)
	}

	limiter.mu.Lock()
	limiter.next = time.Now().Add(50 * time.Millisecond)
	limiter.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	if err := limiter.Wait(ctx); err == nil {
		t.Fatalf("expected context cancellation error")
	}
}
