package veclite

import (
	"fmt"
	"sort"
	"time"
)

// Reserved payload keys for conversation tracking.
const (
	// PayloadKeySessionID identifies which session/conversation a record belongs to.
	PayloadKeySessionID = "_session_id"
	// PayloadKeyTurnNumber is the sequential turn number within a session.
	PayloadKeyTurnNumber = "_turn_number"
	// PayloadKeyRole indicates the role (e.g., "user", "assistant", "system").
	PayloadKeyRole = "_role"
	// PayloadKeyParentChunk links to the parent chunk ID for threaded conversations.
	PayloadKeyParentChunk = "_parent_chunk"
	// PayloadKeyChildChunks contains IDs of child chunks.
	PayloadKeyChildChunks = "_child_chunks"
	// PayloadKeyThreadRoot is the ID of the root chunk in a thread.
	PayloadKeyThreadRoot = "_thread_root"
)

// ConversationTurn represents a single turn in a conversation.
type ConversationTurn struct {
	// SessionID identifies the conversation session.
	SessionID string

	// TurnNumber is the sequential turn number (1-indexed).
	TurnNumber int

	// Role is the speaker role (e.g., "user", "assistant").
	Role string

	// Content is the text content of the turn.
	Content string

	// Vector is the embedding vector. If nil, the collection's embedder is used.
	Vector []float32

	// ParentChunkID links to a parent chunk for threaded conversations.
	ParentChunkID uint64

	// Payload contains additional metadata.
	Payload map[string]any

	// Importance is the importance score (0.0-1.0).
	Importance float32

	// TTL is the time-to-live for this turn.
	TTL time.Duration
}

// InsertTurn inserts a conversation turn with conversation metadata.
// Returns the record ID.
func (c *Collection) InsertTurn(turn ConversationTurn) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}

	if turn.SessionID == "" {
		return 0, fmt.Errorf("veclite: session ID is required")
	}

	// Get or compute embedding
	var vector []float32
	if len(turn.Vector) > 0 {
		vector = turn.Vector
	} else if c.embedder != nil && turn.Content != "" {
		var err error
		vector, err = c.embedder.Embed(turn.Content)
		if err != nil {
			return 0, fmt.Errorf("veclite: embedding failed: %w", err)
		}
	} else if turn.Content != "" {
		return 0, ErrNoEmbedder
	} else {
		return 0, ErrEmptyVector
	}

	// Build payload with conversation metadata
	payload := make(map[string]any)
	if turn.Payload != nil {
		for k, v := range turn.Payload {
			payload[k] = v
		}
	}

	payload[PayloadKeySessionID] = turn.SessionID
	payload[PayloadKeyTurnNumber] = turn.TurnNumber
	if turn.Role != "" {
		payload[PayloadKeyRole] = turn.Role
	}
	if turn.ParentChunkID > 0 {
		payload[PayloadKeyParentChunk] = turn.ParentChunkID
	}

	// Build insert options
	opts := []InsertOption{
		WithContentOption(turn.Content),
	}

	if turn.Importance > 0 {
		opts = append(opts, WithImportance(turn.Importance))
	}

	if turn.TTL > 0 {
		opts = append(opts, WithTTL(turn.TTL))
	}

	id, err := c.InsertWithOptions(vector, payload, opts...)
	if err != nil {
		return 0, err
	}

	// Update parent's child list if this is a reply
	if turn.ParentChunkID > 0 {
		c.mu.Lock()
		if parent, ok := c.records[turn.ParentChunkID]; ok {
			if parent.Payload == nil {
				parent.Payload = make(map[string]any)
			}
			children := getChildIDs(parent.Payload)
			children = append(children, id)
			parent.Payload[PayloadKeyChildChunks] = children

			// Set thread root
			if rootID := getThreadRoot(parent.Payload); rootID > 0 {
				payload[PayloadKeyThreadRoot] = rootID
			} else {
				payload[PayloadKeyThreadRoot] = turn.ParentChunkID
			}
			c.records[id].Payload = payload
		}
		c.mu.Unlock()
	}

	return id, nil
}

// GetThread retrieves all records in a thread starting from the given chunk ID.
// Returns records in chronological order.
func (c *Collection) GetThread(chunkID uint64) ([]*Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Find the root of the thread
	root, ok := c.records[chunkID]
	if !ok {
		return nil, &NotFoundError{Type: "record", ID: fmt.Sprintf("%d", chunkID)}
	}

	// Check if this chunk has a thread root
	rootID := getThreadRoot(root.Payload)
	if rootID > 0 && rootID != chunkID {
		if r, ok := c.records[rootID]; ok {
			root = r
			chunkID = rootID
		}
	}

	// Collect all records in the thread using BFS
	thread := []*Record{root.Clone()}
	visited := map[uint64]bool{chunkID: true}
	queue := []uint64{chunkID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		record := c.records[current]
		if record == nil {
			continue
		}

		// Add children to queue
		for _, childID := range getChildIDs(record.Payload) {
			if !visited[childID] {
				visited[childID] = true
				if child, ok := c.records[childID]; ok {
					thread = append(thread, child.Clone())
					queue = append(queue, childID)
				}
			}
		}
	}

	// Sort by creation time
	sort.Slice(thread, func(i, j int) bool {
		return thread[i].CreatedAt.Before(thread[j].CreatedAt)
	})

	return thread, nil
}

// GetSession retrieves all records belonging to a session.
// Returns records in turn number order.
func (c *Collection) GetSession(sessionID string) ([]*Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var records []*Record
	for _, record := range c.records {
		if record.Payload == nil {
			continue
		}
		if sid, ok := record.Payload[PayloadKeySessionID]; ok && sid == sessionID {
			records = append(records, record.Clone())
		}
	}

	// Sort by turn number, then by creation time
	sort.Slice(records, func(i, j int) bool {
		turnI := getTurnNumber(records[i].Payload)
		turnJ := getTurnNumber(records[j].Payload)
		if turnI != turnJ {
			return turnI < turnJ
		}
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})

	return records, nil
}

// SearchInSession searches for similar vectors within a specific session.
func (c *Collection) SearchInSession(sessionID string, query []float32, opts ...SearchOption) ([]Result, error) {
	// Add session filter
	sessionFilter := Equal(PayloadKeySessionID, sessionID)
	opts = append(opts, WithFilter(sessionFilter))

	return c.Search(query, opts...)
}

// ListSessions returns all unique session IDs in the collection.
func (c *Collection) ListSessions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sessions := make(map[string]bool)
	for _, record := range c.records {
		if record.Payload == nil {
			continue
		}
		if sid, ok := record.Payload[PayloadKeySessionID].(string); ok {
			sessions[sid] = true
		}
	}

	result := make([]string, 0, len(sessions))
	for sid := range sessions {
		result = append(result, sid)
	}
	sort.Strings(result)
	return result
}

// SessionStats returns statistics about a session.
type SessionStats struct {
	// SessionID is the session identifier.
	SessionID string
	// TurnCount is the number of turns in the session.
	TurnCount int
	// FirstTurn is the timestamp of the first turn.
	FirstTurn time.Time
	// LastTurn is the timestamp of the last turn.
	LastTurn time.Time
	// Roles contains the count of turns by role.
	Roles map[string]int
}

// GetSessionStats returns statistics about a session.
func (c *Collection) GetSessionStats(sessionID string) (SessionStats, error) {
	records, err := c.GetSession(sessionID)
	if err != nil {
		return SessionStats{}, err
	}

	if len(records) == 0 {
		return SessionStats{SessionID: sessionID}, nil
	}

	stats := SessionStats{
		SessionID: sessionID,
		TurnCount: len(records),
		FirstTurn: records[0].CreatedAt,
		LastTurn:  records[len(records)-1].CreatedAt,
		Roles:     make(map[string]int),
	}

	for _, r := range records {
		if role, ok := r.Payload[PayloadKeyRole].(string); ok {
			stats.Roles[role]++
		}
	}

	return stats, nil
}

// Helper functions

func getChildIDs(payload map[string]any) []uint64 {
	if payload == nil {
		return nil
	}
	children, ok := payload[PayloadKeyChildChunks]
	if !ok {
		return nil
	}

	switch c := children.(type) {
	case []uint64:
		return c
	case []any:
		result := make([]uint64, 0, len(c))
		for _, v := range c {
			switch id := v.(type) {
			case uint64:
				result = append(result, id)
			case int64:
				result = append(result, uint64(id))
			case float64:
				result = append(result, uint64(id))
			case int:
				result = append(result, uint64(id))
			}
		}
		return result
	}
	return nil
}

func getThreadRoot(payload map[string]any) uint64 {
	if payload == nil {
		return 0
	}
	root, ok := payload[PayloadKeyThreadRoot]
	if !ok {
		return 0
	}

	switch r := root.(type) {
	case uint64:
		return r
	case int64:
		return uint64(r)
	case float64:
		return uint64(r)
	case int:
		return uint64(r)
	}
	return 0
}

func getTurnNumber(payload map[string]any) int {
	if payload == nil {
		return 0
	}
	turn, ok := payload[PayloadKeyTurnNumber]
	if !ok {
		return 0
	}

	switch t := turn.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	}
	return 0
}
