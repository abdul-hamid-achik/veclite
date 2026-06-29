package hnsw

// Delete marks a node as deleted (soft delete).
// The node remains in the graph but is skipped during search.
func (idx *Index) Delete(id uint64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	node, ok := idx.nodes[id]
	if !ok {
		return ErrNotFound
	}

	node.Deleted = true

	// If we just soft-deleted the current entry point, pick a new live one so
	// subsequent inserts/searches don't start from a tombstoned node (which
	// makes searchLayer return nil — an Insert would panic at [0], and Search
	// would silently return empty).
	if idx.entryPoint == id {
		idx.updateEntryPointLocked()
	}
	return nil
}

// Undelete removes the deleted mark from a node.
func (idx *Index) Undelete(id uint64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	node, ok := idx.nodes[id]
	if !ok {
		return ErrNotFound
	}

	node.Deleted = false
	return nil
}

// IsDeleted returns true if a node is marked as deleted.
func (idx *Index) IsDeleted(id uint64) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	node, ok := idx.nodes[id]
	if !ok {
		return false
	}
	return node.Deleted
}

// DeletedCount returns the number of deleted nodes.
func (idx *Index) DeletedCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	count := 0
	for _, node := range idx.nodes {
		if node.Deleted {
			count++
		}
	}
	return count
}

// Compact removes all deleted nodes from the index.
// This is an expensive operation that rebuilds affected parts of the graph.
func (idx *Index) Compact() int {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	deleted := make([]uint64, 0)
	for id, node := range idx.nodes {
		if node.Deleted {
			deleted = append(deleted, id)
		}
	}

	if len(deleted) == 0 {
		return 0
	}

	// Remove deleted nodes and their connections
	for _, id := range deleted {
		idx.removeNodeLocked(id)
	}

	return len(deleted)
}

// removeNodeLocked removes a node and all its connections.
// Must be called with the lock held.
func (idx *Index) removeNodeLocked(id uint64) {
	node, ok := idx.nodes[id]
	if !ok {
		return
	}

	// Remove all connections to this node
	for layer := 0; layer <= node.Level; layer++ {
		for _, neighborID := range node.Neighbors[layer] {
			neighborNode := idx.nodes[neighborID]
			if neighborNode != nil {
				neighborNode.RemoveNeighbor(layer, id)
			}
		}
	}

	// Remove node and vector (only from internal map if using internal storage)
	delete(idx.nodes, id)
	if idx.vectorProvider == nil {
		delete(idx.vectors, id)
	}

	// Update entry point if needed
	if idx.entryPoint == id {
		idx.updateEntryPointLocked()
	}
}

// updateEntryPointLocked finds a new entry point after the current one was deleted.
// Must be called with the lock held.
func (idx *Index) updateEntryPointLocked() {
	if len(idx.nodes) == 0 {
		idx.entryPoint = 0
		idx.maxLevel = -1
		return
	}

	// Find the node with the highest level
	maxLevel := -1
	var newEntry uint64
	for id, node := range idx.nodes {
		if !node.Deleted && node.Level > maxLevel {
			maxLevel = node.Level
			newEntry = id
		}
	}

	idx.entryPoint = newEntry
	idx.maxLevel = maxLevel
}

// HardDelete removes a node completely from the index (not recommended).
// This can leave the graph in a suboptimal state. Use Compact() instead.
func (idx *Index) HardDelete(id uint64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, ok := idx.nodes[id]; !ok {
		return ErrNotFound
	}

	idx.removeNodeLocked(id)
	return nil
}
