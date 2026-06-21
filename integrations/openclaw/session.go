package openclaw

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ImportOptions configures session import behavior.
type ImportOptions struct {
	// DefaultImportance is the importance for imported memories.
	// Defaults to 0.5.
	DefaultImportance float32

	// DefaultTTL is the TTL for imported memories.
	// Zero means no expiration.
	DefaultTTL time.Duration

	// Tags are tags to add to all imported memories.
	Tags []string

	// FilterRole filters messages by role (e.g., "assistant", "user").
	// Empty means import all roles.
	FilterRole string

	// MinContentLength skips messages shorter than this.
	MinContentLength int
}

// SessionMessage represents a message in a session file.
type SessionMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SessionFile represents the structure of an OpenClaw session file.
type SessionFile struct {
	ID        string           `json:"id"`
	Title     string           `json:"title,omitempty"`
	CreatedAt string           `json:"created_at,omitempty"`
	Messages  []SessionMessage `json:"messages"`
}

// ImportSession imports memories from an OpenClaw session file.
// Supports both JSON (full session) and JSONL (streaming) formats.
// Returns the number of memories imported.
func (m *Memory) ImportSession(sessionPath string, opts ImportOptions) (int, error) {
	if m.embedder == nil {
		return 0, fmt.Errorf("openclaw: embedder required for session import")
	}

	file, err := os.Open(sessionPath)
	if err != nil {
		return 0, fmt.Errorf("openclaw: failed to open session file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Set defaults
	if opts.DefaultImportance <= 0 || opts.DefaultImportance > 1 {
		opts.DefaultImportance = 0.5
	}

	// Try to read as JSON first
	var session SessionFile
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&session); err == nil && len(session.Messages) > 0 {
		return m.importMessages(session.Messages, opts)
	}

	// Reset file and try JSONL format
	if _, err := file.Seek(0, 0); err != nil {
		return 0, fmt.Errorf("openclaw: failed to seek file: %w", err)
	}

	var messages []SessionMessage
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg SessionMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // Skip invalid lines
		}
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("openclaw: error reading session file: %w", err)
	}

	if len(messages) == 0 {
		return 0, fmt.Errorf("openclaw: no messages found in session file")
	}

	return m.importMessages(messages, opts)
}

// importMessages imports a slice of session messages.
func (m *Memory) importMessages(messages []SessionMessage, opts ImportOptions) (int, error) {
	imported := 0

	for _, msg := range messages {
		// Apply role filter
		if opts.FilterRole != "" && msg.Role != opts.FilterRole {
			continue
		}

		// Apply content length filter
		if len(msg.Content) < opts.MinContentLength {
			continue
		}

		// Build tags
		tags := make([]string, 0, len(opts.Tags)+1)
		tags = append(tags, opts.Tags...)
		tags = append(tags, "imported")
		tags = append(tags, fmt.Sprintf("role:%s", msg.Role))

		// Build metadata
		metadata := map[string]any{
			"role":   msg.Role,
			"source": "session_import",
		}
		if msg.Timestamp != "" {
			metadata["original_timestamp"] = msg.Timestamp
		}

		// Remember the message
		_, err := m.Remember(msg.Content, RememberOptions{
			Importance: opts.DefaultImportance,
			Tags:       tags,
			TTL:        opts.DefaultTTL,
			Metadata:   metadata,
		})
		if err != nil {
			return imported, fmt.Errorf("openclaw: failed to import message: %w", err)
		}

		imported++
	}

	return imported, nil
}

// ImportText imports plain text content, splitting it into chunks.
// Each chunk becomes a separate memory.
// Returns the number of memories imported.
func (m *Memory) ImportText(content string, chunkSize int, opts ImportOptions) (int, error) {
	if m.embedder == nil {
		return 0, fmt.Errorf("openclaw: embedder required for text import")
	}

	if chunkSize <= 0 {
		chunkSize = 1000 // Default chunk size
	}

	// Set defaults
	if opts.DefaultImportance <= 0 || opts.DefaultImportance > 1 {
		opts.DefaultImportance = 0.5
	}

	// Split content into chunks
	chunks := splitIntoChunks(content, chunkSize)
	imported := 0

	tags := make([]string, 0, len(opts.Tags)+1)
	tags = append(tags, opts.Tags...)
	tags = append(tags, "imported")

	for i, chunk := range chunks {
		if len(chunk) < opts.MinContentLength {
			continue
		}

		metadata := map[string]any{
			"source":       "text_import",
			"chunk_index":  i,
			"total_chunks": len(chunks),
		}

		_, err := m.Remember(chunk, RememberOptions{
			Importance: opts.DefaultImportance,
			Tags:       tags,
			TTL:        opts.DefaultTTL,
			Metadata:   metadata,
		})
		if err != nil {
			return imported, fmt.Errorf("openclaw: failed to import chunk %d: %w", i, err)
		}

		imported++
	}

	return imported, nil
}

// splitIntoChunks splits text into chunks of approximately the given size.
// It tries to split on sentence or paragraph boundaries.
func splitIntoChunks(content string, maxSize int) []string {
	if len(content) <= maxSize {
		return []string{content}
	}

	var chunks []string
	runes := []rune(content)

	for start := 0; start < len(runes); {
		end := start + maxSize
		if end > len(runes) {
			end = len(runes)
		}

		// Try to find a good break point
		if end < len(runes) {
			// Look for paragraph break
			for i := end; i > start+maxSize/2; i-- {
				if runes[i] == '\n' && i+1 < len(runes) && runes[i+1] == '\n' {
					end = i
					break
				}
			}
			// Look for sentence break if no paragraph break found
			if end == start+maxSize {
				for i := end; i > start+maxSize/2; i-- {
					if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
						if i+1 < len(runes) && (runes[i+1] == ' ' || runes[i+1] == '\n') {
							end = i + 1
							break
						}
					}
				}
			}
		}

		chunk := string(runes[start:end])
		chunks = append(chunks, chunk)
		start = end

		// Skip leading whitespace for next chunk
		for start < len(runes) && (runes[start] == ' ' || runes[start] == '\n') {
			start++
		}
	}

	return chunks
}
