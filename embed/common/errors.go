// Package common provides shared utilities for embedder implementations.
package common

import (
	"errors"
	"fmt"
)

// Common errors for embedder implementations.
var (
	// ErrClosed is returned when operations are attempted on a closed embedder.
	ErrClosed = errors.New("embedder: closed")

	// ErrNoAPIKey is returned when no API key is provided for API-based embedders.
	ErrNoAPIKey = errors.New("embedder: no API key provided")

	// ErrRateLimited is returned when the API rate limit is exceeded.
	ErrRateLimited = errors.New("embedder: rate limited")

	// ErrEmptyInput is returned when empty text is provided for embedding.
	ErrEmptyInput = errors.New("embedder: empty input")

	// ErrServerUnavailable is returned when the embedding server is unavailable.
	ErrServerUnavailable = errors.New("embedder: server unavailable")
)

// APIError represents an error from an embedding API.
type APIError struct {
	StatusCode int
	Message    string
	Provider   string
	Retryable  bool
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("%s API error (status %d): %s", e.Provider, e.StatusCode, e.Message)
}

// IsRetryable returns true if the error is retryable.
func (e *APIError) IsRetryable() bool {
	return e.Retryable
}

// NewAPIError creates a new APIError.
func NewAPIError(provider string, statusCode int, message string) *APIError {
	retryable := statusCode == 429 || statusCode >= 500
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		Provider:   provider,
		Retryable:  retryable,
	}
}
