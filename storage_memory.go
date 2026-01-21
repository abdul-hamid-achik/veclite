package veclite

// MemoryStorage is an in-memory storage implementation.
// Data is not persisted and will be lost when the database is closed.
type MemoryStorage struct {
	snapshot *DatabaseSnapshot
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

// Load returns the stored snapshot or nil if none exists.
func (m *MemoryStorage) Load() (*DatabaseSnapshot, error) {
	return m.snapshot, nil
}

// Save stores the snapshot in memory.
func (m *MemoryStorage) Save(snapshot *DatabaseSnapshot) error {
	m.snapshot = snapshot
	return nil
}

// Close is a no-op for memory storage.
func (m *MemoryStorage) Close() error {
	return nil
}

// Ensure MemoryStorage implements Storage.
var _ Storage = (*MemoryStorage)(nil)
