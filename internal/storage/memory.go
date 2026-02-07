package storage

// Memory is an in-memory storage implementation.
// Data is not persisted and will be lost when the database is closed.
type Memory struct {
	snapshot *DatabaseSnapshot
}

// NewMemory creates a new in-memory storage.
func NewMemory() *Memory {
	return &Memory{}
}

// Load returns the stored snapshot or nil if none exists.
func (m *Memory) Load() (*DatabaseSnapshot, error) {
	return m.snapshot, nil
}

// Save stores the snapshot in memory.
func (m *Memory) Save(snapshot *DatabaseSnapshot) error {
	m.snapshot = snapshot
	return nil
}

// Close is a no-op for memory storage.
func (m *Memory) Close() error {
	return nil
}

// Ensure Memory implements Backend.
var _ Backend = (*Memory)(nil)
