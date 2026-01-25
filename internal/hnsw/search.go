package hnsw

// SearchResult represents a single search result.
type SearchResult struct {
	ID       uint64
	Distance float32
}

// Search finds the k nearest neighbors to the query vector.
func (idx *Index) Search(query []float32, k int) ([]SearchResult, error) {
	return idx.SearchWithEf(query, k, idx.config.EfSearch)
}

// SearchWithEf finds the k nearest neighbors using the specified ef parameter.
func (idx *Index) SearchWithEf(query []float32, k int, ef int) ([]SearchResult, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.nodes) == 0 {
		return nil, ErrEmptyIndex
	}

	if idx.dimension > 0 && len(query) != idx.dimension {
		return nil, &DimensionError{Expected: idx.dimension, Got: len(query)}
	}

	// Ensure ef >= k for good recall
	if ef < k {
		ef = k
	}

	// Start from entry point
	currID := idx.entryPoint

	// Navigate from top level down to level 1
	for l := idx.maxLevel; l > 0; l-- {
		results := idx.searchLayer(query, currID, 1, l)
		if len(results) > 0 {
			currID = results[0].ID
		}
	}

	// Search at layer 0 with ef
	results := idx.searchLayer(query, currID, ef, 0)

	// Convert to SearchResult and limit to k
	if len(results) > k {
		results = results[:k]
	}

	output := make([]SearchResult, len(results))
	for i, item := range results {
		output[i] = SearchResult(item)
	}

	return output, nil
}

// SearchStats holds statistics about a search operation.
type SearchStats struct {
	NodesVisited  int
	LayersVisited int
}

// SearchWithStats finds the k nearest neighbors and returns search statistics.
func (idx *Index) SearchWithStats(query []float32, k int, ef int) ([]SearchResult, SearchStats, error) {
	if len(query) == 0 {
		return nil, SearchStats{}, ErrEmptyVector
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.nodes) == 0 {
		return nil, SearchStats{}, ErrEmptyIndex
	}

	if idx.dimension > 0 && len(query) != idx.dimension {
		return nil, SearchStats{}, &DimensionError{Expected: idx.dimension, Got: len(query)}
	}

	// Ensure ef >= k for good recall
	if ef < k {
		ef = k
	}

	stats := SearchStats{}
	visited := make(map[uint64]bool)

	// Start from entry point
	currID := idx.entryPoint

	// Navigate from top level down to level 1
	for l := idx.maxLevel; l > 0; l-- {
		results := idx.searchLayerTracked(query, currID, 1, l, visited)
		stats.LayersVisited++
		if len(results) > 0 {
			currID = results[0].ID
		}
	}

	// Search at layer 0 with ef
	results := idx.searchLayerTracked(query, currID, ef, 0, visited)
	stats.LayersVisited++
	stats.NodesVisited = len(visited)

	// Convert to SearchResult and limit to k
	if len(results) > k {
		results = results[:k]
	}

	output := make([]SearchResult, len(results))
	for i, item := range results {
		output[i] = SearchResult(item)
	}

	return output, stats, nil
}

// searchLayerTracked performs search on a layer while tracking visited nodes.
func (idx *Index) searchLayerTracked(query []float32, entryID uint64, ef int, layer int, visited map[uint64]bool) []Item {
	entryNode := idx.nodes[entryID]
	if entryNode == nil || entryNode.Deleted {
		return nil
	}

	// Initialize with entry point
	entryDist := idx.distFunc(query, idx.vectors[entryID])
	candidates := NewCandidateSet(ef, idx.higherBetter)
	candidates.Add(entryID, entryDist)
	visited[entryID] = true

	// Greedy search
	for candidates.HasCandidates() {
		curr := candidates.PopNearest()

		// Early termination: if current is worse than worst result, stop
		if candidates.ResultsFull() {
			if idx.higherBetter {
				if curr.Distance < candidates.WorstDistance() {
					break
				}
			} else {
				if curr.Distance > candidates.WorstDistance() {
					break
				}
			}
		}

		// Explore neighbors
		node := idx.nodes[curr.ID]
		if node == nil {
			continue
		}

		neighbors := node.GetNeighbors(layer)
		for _, neighborID := range neighbors {
			if candidates.IsVisited(neighborID) {
				continue
			}

			neighborNode := idx.nodes[neighborID]
			if neighborNode == nil || neighborNode.Deleted {
				continue
			}

			visited[neighborID] = true
			dist := idx.distFunc(query, idx.vectors[neighborID])

			// Add if results not full or better than worst
			if !candidates.ResultsFull() {
				candidates.Add(neighborID, dist)
			} else {
				if idx.higherBetter {
					if dist > candidates.WorstDistance() {
						candidates.Add(neighborID, dist)
					}
				} else {
					if dist < candidates.WorstDistance() {
						candidates.Add(neighborID, dist)
					}
				}
			}
		}
	}

	return candidates.Results()
}

// KNNBruteForce performs brute-force k-NN search for comparison/verification.
func (idx *Index) KNNBruteForce(query []float32, k int) ([]SearchResult, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.nodes) == 0 {
		return nil, ErrEmptyIndex
	}

	if idx.dimension > 0 && len(query) != idx.dimension {
		return nil, &DimensionError{Expected: idx.dimension, Got: len(query)}
	}

	// Calculate all distances
	type result struct {
		id   uint64
		dist float32
	}
	results := make([]result, 0, len(idx.nodes))

	for id, node := range idx.nodes {
		if node.Deleted {
			continue
		}
		dist := idx.distFunc(query, idx.vectors[id])
		results = append(results, result{id: id, dist: dist})
	}

	// Sort by distance
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if idx.higherBetter {
				if results[i].dist < results[j].dist {
					results[i], results[j] = results[j], results[i]
				}
			} else {
				if results[i].dist > results[j].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	}

	// Return top k
	if len(results) > k {
		results = results[:k]
	}

	output := make([]SearchResult, len(results))
	for i, r := range results {
		output[i] = SearchResult{
			ID:       r.id,
			Distance: r.dist,
		}
	}

	return output, nil
}
