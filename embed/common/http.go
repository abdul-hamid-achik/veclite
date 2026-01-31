package common

import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

// DefaultTimeout is the default HTTP timeout for embedding requests.
const DefaultTimeout = 30 * time.Second

// DefaultHTTPClient creates an HTTP client with sensible defaults for embedding APIs.
func DefaultHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// RetryConfig configures retry behavior for HTTP requests.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}
}

// DoWithRetry executes an HTTP request with exponential backoff retry for retryable errors.
// It retries on 429 (rate limit) and 5xx errors.
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, cfg RetryConfig) (*http.Response, error) {
	var lastErr error
	backoff := cfg.InitialBackoff

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait with jitter
			jitter := time.Duration(rand.Float64() * float64(backoff) * 0.1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff + jitter):
			}
			backoff = min(time.Duration(float64(backoff)*cfg.BackoffFactor), cfg.MaxBackoff)
		}

		// Clone request for retry (body needs to be re-read)
		reqClone := req.Clone(ctx)
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			reqClone.Body = body
		}

		resp, err := client.Do(reqClone)
		if err != nil {
			lastErr = err
			continue
		}

		// Check if we should retry
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = NewAPIError("http", resp.StatusCode, "retryable error")
			continue
		}

		return resp, nil
	}

	return nil, lastErr
}

// IsRetryableStatusCode returns true if the status code is retryable.
func IsRetryableStatusCode(statusCode int) bool {
	return statusCode == 429 || statusCode >= 500
}
