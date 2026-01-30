package veclite

import "time"

// InsertOption configures record insertion behavior.
type InsertOption interface {
	apply(*insertConfig)
}

// insertConfig holds the insert configuration.
type insertConfig struct {
	ttl        time.Duration
	expiresAt  time.Time
	importance float32
	content    string
}

// defaultInsertConfig returns the default insert configuration.
func defaultInsertConfig() *insertConfig {
	return &insertConfig{
		importance: 0.5, // Default importance is middle of the range
	}
}

// insertOptionFunc is a function adapter for InsertOption.
type insertOptionFunc func(*insertConfig)

func (f insertOptionFunc) apply(c *insertConfig) {
	f(c)
}

// WithTTL sets a time-to-live duration for the record.
// The record will expire after this duration from the time of insertion.
func WithTTL(d time.Duration) InsertOption {
	return insertOptionFunc(func(c *insertConfig) {
		if d > 0 {
			c.ttl = d
		}
	})
}

// WithExpiresAt sets an explicit expiration time for the record.
// This takes precedence over WithTTL if both are specified.
func WithExpiresAt(t time.Time) InsertOption {
	return insertOptionFunc(func(c *insertConfig) {
		c.expiresAt = t
	})
}

// WithImportance sets the importance score for the record.
// Value should be between 0.0 and 1.0. Values outside this range are clamped.
func WithImportance(score float32) InsertOption {
	return insertOptionFunc(func(c *insertConfig) {
		if score < 0 {
			score = 0
		} else if score > 1 {
			score = 1
		}
		c.importance = score
	})
}

// WithContentOption sets the content field for the record.
// This is an alternative to using InsertDocument.
func WithContentOption(content string) InsertOption {
	return insertOptionFunc(func(c *insertConfig) {
		c.content = content
	})
}

// computeExpiresAt returns the expiration time based on the insert config.
func (c *insertConfig) computeExpiresAt() time.Time {
	// Explicit expiration takes precedence
	if !c.expiresAt.IsZero() {
		return c.expiresAt
	}
	// Otherwise use TTL if set
	if c.ttl > 0 {
		return time.Now().Add(c.ttl)
	}
	// Zero means no expiration
	return time.Time{}
}
