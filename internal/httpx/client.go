package httpx

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Client wraps http.Client with global rate limiting and retry logic.
type Client struct {
	client       *http.Client
	limiter      *RateLimiter
	reqInterval  time.Duration
	limitWait    time.Duration
	maxRetries   int
	jitterFactor float64

	randSrc *rand.Rand
	randMu  sync.Mutex
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithMaxRetries overrides the default retry count.
func WithMaxRetries(n int) ClientOption {
	return func(c *Client) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithHTTPClient injects a custom http.Client.
func WithHTTPClient(h *http.Client) ClientOption {
	return func(c *Client) {
		if h != nil {
			c.client = h
		}
	}
}

// WithJitterFactor overrides jitter amplitude (0.1 == ±10%).
func WithJitterFactor(f float64) ClientOption {
	return func(c *Client) {
		if f >= 0 {
			c.jitterFactor = f
		}
	}
}

// NewClient builds a Client with sensible defaults.
func NewClient(reqInterval, limitWait time.Duration, opts ...ClientOption) *Client {
	if reqInterval <= 0 {
		reqInterval = 600 * time.Millisecond
	}
	if limitWait <= 0 {
		limitWait = 60 * time.Second
	}
	c := &Client{
		client:       &http.Client{Timeout: 30 * time.Second},
		limiter:      NewRateLimiter(reqInterval),
		reqInterval:  reqInterval,
		limitWait:    limitWait,
		maxRetries:   5,
		jitterFactor: 0.1,
		randSrc:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.limiter == nil {
		c.limiter = NewRateLimiter(reqInterval)
	}
	return c
}

// Do issues the HTTP request with retry control. The caller is responsible for closing resp.Body.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		clone := cloneRequest(ctx, req)
		resp, err := c.client.Do(clone)
		if err != nil {
			lastErr = err
			if attempt == c.maxRetries {
				return nil, lastErr
			}
			if err := c.sleepWithBackoff(ctx, attempt); err != nil {
				return nil, err
			}
			continue
		}

		switch resp.StatusCode {
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
			wait := c.retryAfterDuration(resp)
			resp.Body.Close()
			if attempt == c.maxRetries {
				return nil, fmt.Errorf("retries exhausted after %d attempts (status %s)", attempt+1, resp.Status)
			}
			if err := c.sleep(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %s", resp.Status)
			resp.Body.Close()
			if attempt == c.maxRetries {
				return nil, lastErr
			}
			if err := c.sleepWithBackoff(ctx, attempt); err != nil {
				return nil, err
			}
			continue
		}

		return resp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("request failed after %d retries", c.maxRetries)
	}
	return nil, lastErr
}

func (c *Client) sleepWithBackoff(ctx context.Context, attempt int) error {
	if attempt < 0 {
		attempt = 0
	}
	multiplier := 1 << attempt
	if multiplier < 1 {
		multiplier = 1
	}
	backoff := c.reqInterval * time.Duration(multiplier)
	if backoff < c.reqInterval {
		backoff = c.reqInterval
	}
	if backoff > c.limitWait {
		backoff = c.limitWait
	}
	return c.sleep(ctx, backoff)
}

func (c *Client) sleep(ctx context.Context, base time.Duration) error {
	if base <= 0 {
		base = c.reqInterval
	}
	jittered := c.applyJitter(base)
	timer := time.NewTimer(jittered)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) applyJitter(d time.Duration) time.Duration {
	if d <= 0 || c.jitterFactor <= 0 {
		return d
	}
	c.randMu.Lock()
	defer c.randMu.Unlock()
	delta := (c.randSrc.Float64()*2 - 1) * c.jitterFactor
	scale := 1 + delta
	if scale < 0.1 {
		scale = 0.1
	}
	return time.Duration(float64(d) * scale)
}

func (c *Client) retryAfterDuration(resp *http.Response) time.Duration {
	header := resp.Header.Get("Retry-After")
	if header == "" {
		if c.limitWait > c.reqInterval {
			return c.limitWait
		}
		return c.reqInterval
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		wait := time.Duration(seconds) * time.Second
		if wait < c.reqInterval {
			wait = c.reqInterval
		}
		return wait
	}
	if t, err := http.ParseTime(header); err == nil {
		wait := time.Until(t)
		if wait < c.reqInterval {
			wait = c.reqInterval
		}
		return wait
	}
	// Fallback when header is malformed.
	return c.limitWait
}

func cloneRequest(ctx context.Context, req *http.Request) *http.Request {
	clone := req.Clone(ctx)
	clone.Header = req.Header.Clone()
	return clone
}
