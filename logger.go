package veclite

// Logger is the interface for structured logging in VecLite.
// Implementations can bridge to any logging library (slog, zap, zerolog, etc.).
type Logger interface {
	// Debug logs a debug message with key-value pairs.
	Debug(msg string, keysAndValues ...any)

	// Info logs an informational message with key-value pairs.
	Info(msg string, keysAndValues ...any)

	// Error logs an error message with key-value pairs.
	Error(msg string, keysAndValues ...any)
}

// NopLogger is a no-op logger that discards all messages.
// This is the default logger used when none is configured, ensuring zero overhead.
type NopLogger struct{}

func (NopLogger) Debug(string, ...any) {}
func (NopLogger) Info(string, ...any)  {}
func (NopLogger) Error(string, ...any) {}

// WithLogger sets a logger for the database.
// Pass nil or NopLogger{} to disable logging (default).
func WithLogger(l Logger) Option {
	return dbOptionFunc(func(c *dbConfig) {
		c.logger = l
	})
}
