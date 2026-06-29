package hnsw

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// VectorProvider is a function that retrieves a vector by ID.
// Used to allow the index to reference vectors stored externally,
// avoiding duplicate storage.
type VectorProvider func(id uint64) ([]float32, bool)

// Index is the HNSW index structure.
type Index struct {
	config         Config
	nodes          map[uint64]*Node
	vectors        map[uint64][]float32
	vectorProvider VectorProvider // if set, used instead of vectors map
	entryPoint     uint64
	maxLevel       int
	dimension      int
	distFunc       floats.DistanceFunc
	higherBetter   bool
	mu             sync.RWMutex
	rng            *rand.Rand
}

// New creates a new HNSW index with the given configuration.
func New(config Config, dimension int, distanceType floats.DistanceType) *Index {
	if !config.Validate() {
		config = DefaultConfig()
	}

	return &Index{
		config:       config,
		nodes:        make(map[uint64]*Node),
		vectors:      make(map[uint64][]float32),
		entryPoint:   0,
		maxLevel:     -1,
		dimension:    dimension,
		distFunc:     floats.GetDistanceFunc(distanceType),
		higherBetter: floats.IsHigherBetter(distanceType),
		rng:          rand.New(rand.NewSource(42)),
	}
}

// SetVectorProvider sets an external vector provider.
// When set, the index will look up vectors through this provider
// instead of its internal map, avoiding duplicate storage.
// The provider must be safe for concurrent use.
func (idx *Index) SetVectorProvider(provider VectorProvider) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.vectorProvider = provider
}

// ClearInternalVectors removes all vectors from the internal map.
// Use this after setting a VectorProvider to free the duplicate memory.
func (idx *Index) ClearInternalVectors() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.vectors = make(map[uint64][]float32)
}

// Clear removes all nodes and resets the index to an empty state.
func (idx *Index) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.nodes = make(map[uint64]*Node)
	idx.vectors = make(map[uint64][]float32)
	idx.entryPoint = 0
	idx.maxLevel = -1
}

// getVector retrieves a vector by ID, using the provider if set,
// otherwise falling back to the internal vectors map.
func (idx *Index) getVector(id uint64) ([]float32, bool) {
	if idx.vectorProvider != nil {
		return idx.vectorProvider(id)
	}
	v, ok := idx.vectors[id]
	return v, ok
}

// Count returns the number of nodes in the index.
func (idx *Index) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.nodes)
}

// Dimension returns the vector dimension.
func (idx *Index) Dimension() int {
	return idx.dimension
}

// Config returns the index configuration.
func (idx *Index) Config() Config {
	return idx.config
}

// randomLevel generates a random level for a new node using exponential distribution.
func (idx *Index) randomLevel() int {
	// Initialize rng if nil (happens after deserialization from gob)
	if idx.rng == nil {
		idx.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	level := 0
	for idx.rng.Float64() < idx.config.ML && level < 32 {
		level++
	}
	return level
}

// distance computes distance between two nodes.
func (idx *Index) distance(id1, id2 uint64) float32 {
	v1, ok1 := idx.getVector(id1)
	v2, ok2 := idx.getVector(id2)
	if !ok1 || !ok2 {
		if idx.higherBetter {
			return float32(math.Inf(-1))
		}
		return float32(math.Inf(1))
	}
	return idx.distFunc(v1, v2)
}

// Insert adds a vector to the index with the given ID.
func (idx *Index) Insert(id uint64, vector []float32) error {
	if len(vector) == 0 {
		return ErrEmptyVector
	}
	if idx.dimension > 0 && len(vector) != idx.dimension {
		return &DimensionError{Expected: idx.dimension, Got: len(vector)}
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Set dimension on first insert
	if idx.dimension == 0 {
		idx.dimension = len(vector)
	}

	// Check if already exists
	if _, exists := idx.nodes[id]; exists {
		return &DuplicateError{ID: id}
	}

	// Generate random level
	level := idx.randomLevel()

	// Create node
	node := NewNode(id, level)

	// Store vector only if no external provider is set
	if idx.vectorProvider == nil {
		vec := make([]float32, len(vector))
		copy(vec, vector)
		idx.vectors[id] = vec
	}

	// First node becomes the entry point.
	if len(idx.nodes) == 0 {
		idx.nodes[id] = node
		idx.entryPoint = id
		idx.maxLevel = level
		return nil
	}

	// The entry point may have been soft-deleted. A deleted entry point makes
	// searchLayer return nil (and would panic at [0] below), so pick a new
	// live entry point — the alive node with the highest level — before
	// navigating. If no live nodes remain, this insert re-seeds the graph.
	if ep := idx.nodes[idx.entryPoint]; ep == nil || ep.Deleted {
		newEP, newLvl, ok := idx.pickLiveEntryPoint()
		if !ok {
			idx.nodes[id] = node
			idx.entryPoint = id
			idx.maxLevel = level
			return nil
		}
		idx.entryPoint = newEP
		idx.maxLevel = newLvl
	}

	// Find entry point for insertion
	currID := idx.entryPoint

	// Navigate from top level down to the node's level + 1
	for l := idx.maxLevel; l > level; l-- {
		currID = idx.searchLayer(vector, currID, 1, l)[0].ID
	}

	// Insert at each level from node's level down to 0
	for l := min(level, idx.maxLevel); l >= 0; l-- {
		// Find ef_construction nearest neighbors
		neighbors := idx.searchLayer(vector, currID, idx.config.EfConstruction, l)

		// Select M best neighbors
		maxConn := idx.config.M
		if l == 0 {
			maxConn = idx.config.Mmax
		}
		selectedNeighbors := idx.selectNeighbors(neighbors, maxConn)

		// Connect new node to neighbors
		for _, neighbor := range selectedNeighbors {
			node.AddNeighbor(l, neighbor.ID)

			// Add reverse connection
			neighborNode := idx.nodes[neighbor.ID]
			neighborNode.AddNeighbor(l, id)

			// Prune neighbor if it has too many connections
			idx.pruneConnections(neighborNode, l)
		}

		// Use best neighbor as entry point for next level
		if len(neighbors) > 0 {
			currID = neighbors[0].ID
		}
	}

	idx.nodes[id] = node

	// Update entry point if new node has higher level
	if level > idx.maxLevel {
		idx.entryPoint = id
		idx.maxLevel = level
	}

	return nil
}

// pickLiveEntryPoint returns the live (non-deleted) node with the highest
// level, to use as a new entry point after the current one was soft-deleted.
// Returns ok=false if there are no live nodes left (the graph is entirely
// tombstones). Callers must hold idx.mu.
func (idx *Index) pickLiveEntryPoint() (id uint64, level int, ok bool) {
	for nid, n := range idx.nodes {
		if n == nil || n.Deleted {
			continue
		}
		if !ok || n.Level > level {
			id, level, ok = nid, n.Level, true
		}
	}
	return
}

// searchLayer performs greedy search on a single layer.
func (idx *Index) searchLayer(query []float32, entryID uint64, ef int, layer int) []Item {
	entryNode := idx.nodes[entryID]
	if entryNode == nil || entryNode.Deleted {
		return nil
	}

	// Initialize with entry point
	entryVec, _ := idx.getVector(entryID)
	entryDist := idx.distFunc(query, entryVec)
	candidates := NewCandidateSet(ef, idx.higherBetter)
	candidates.Add(entryID, entryDist)

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

			neighborVec, vecOk := idx.getVector(neighborID)
			if !vecOk {
				continue
			}
			dist := idx.distFunc(query, neighborVec)

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

// selectNeighbors selects the best M neighbors from candidates.
// When UseHeuristic is true, uses the diversity-preserving heuristic from
// the HNSW paper (Algorithm 4) which favors candidates that cover different
// regions of the space. Otherwise uses simple distance-based truncation.
func (idx *Index) selectNeighbors(candidates []Item, m int) []Item {
	if len(candidates) <= m {
		return candidates
	}

	if idx.config.UseHeuristic {
		return idx.selectNeighborsHeuristic(candidates, m)
	}
	return idx.selectNeighborsSimple(candidates, m)
}

// selectNeighborsSimple selects neighbors by sorting by distance and taking top M.
func (idx *Index) selectNeighborsSimple(candidates []Item, m int) []Item {
	sorted := make([]Item, len(candidates))
	copy(sorted, candidates)

	if idx.higherBetter {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Distance > sorted[j].Distance
		})
	} else {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Distance < sorted[j].Distance
		})
	}

	return sorted[:m]
}

// selectNeighborsHeuristic implements Algorithm 4 from the HNSW paper.
// It selects diverse neighbors: for each candidate, it checks whether it is closer
// to the query than to any already-selected neighbor. If so, it is selected.
// This prevents redundant connections that cover similar regions of the space.
func (idx *Index) selectNeighborsHeuristic(candidates []Item, m int) []Item {
	// Sort candidates by distance to query (best first)
	sorted := make([]Item, len(candidates))
	copy(sorted, candidates)

	if idx.higherBetter {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Distance > sorted[j].Distance
		})
	} else {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Distance < sorted[j].Distance
		})
	}

	selected := make([]Item, 0, m)

	for _, candidate := range sorted {
		if len(selected) >= m {
			break
		}

		// Check if this candidate is closer to the query than to any selected neighbor
		good := true
		for _, sel := range selected {
			dist := idx.distance(candidate.ID, sel.ID)
			if idx.higherBetter {
				// For similarity metrics, candidate is redundant if it's more similar
				// to an already-selected neighbor than to the query
				if dist > candidate.Distance {
					good = false
					break
				}
			} else {
				// For distance metrics, candidate is redundant if it's closer
				// to an already-selected neighbor than to the query
				if dist < candidate.Distance {
					good = false
					break
				}
			}
		}

		if good {
			selected = append(selected, candidate)
		}
	}

	// If heuristic didn't fill the quota, add remaining candidates by distance
	if len(selected) < m {
		selectedSet := make(map[uint64]bool, len(selected))
		for _, s := range selected {
			selectedSet[s.ID] = true
		}
		for _, candidate := range sorted {
			if len(selected) >= m {
				break
			}
			if !selectedSet[candidate.ID] {
				selected = append(selected, candidate)
			}
		}
	}

	return selected
}

// pruneConnections removes excess connections if node has too many neighbors.
// Uses heuristic selection when UseHeuristic is enabled.
func (idx *Index) pruneConnections(node *Node, layer int) {
	maxConn := idx.config.M
	if layer == 0 {
		maxConn = idx.config.Mmax
	}

	neighbors := node.GetNeighbors(layer)
	if len(neighbors) <= maxConn {
		return
	}

	// Calculate distances
	type neighborDist struct {
		id   uint64
		dist float32
	}

	dists := make([]neighborDist, len(neighbors))
	for i, nid := range neighbors {
		dists[i] = neighborDist{id: nid, dist: idx.distance(node.ID, nid)}
	}

	if idx.config.UseHeuristic {
		// Use heuristic selection for pruning as well
		items := make([]Item, len(dists))
		for i, d := range dists {
			items[i] = Item{ID: d.id, Distance: d.dist}
		}
		selected := idx.selectNeighborsHeuristic(items, maxConn)

		newNeighbors := make([]uint64, len(selected))
		for i, s := range selected {
			newNeighbors[i] = s.ID
		}
		node.SetNeighbors(layer, newNeighbors)

		// Remove reverse connections for pruned nodes
		selectedSet := make(map[uint64]bool, len(selected))
		for _, s := range selected {
			selectedSet[s.ID] = true
		}
		for _, d := range dists {
			if !selectedSet[d.id] {
				prunedNode := idx.nodes[d.id]
				if prunedNode != nil {
					prunedNode.RemoveNeighbor(layer, node.ID)
				}
			}
		}
	} else {
		// Sort by distance (keep best connections)
		if idx.higherBetter {
			sort.Slice(dists, func(i, j int) bool {
				return dists[i].dist > dists[j].dist
			})
		} else {
			sort.Slice(dists, func(i, j int) bool {
				return dists[i].dist < dists[j].dist
			})
		}

		newNeighbors := make([]uint64, maxConn)
		for i := 0; i < maxConn; i++ {
			newNeighbors[i] = dists[i].id
		}
		node.SetNeighbors(layer, newNeighbors)

		// Remove reverse connections for pruned nodes
		for i := maxConn; i < len(dists); i++ {
			prunedNode := idx.nodes[dists[i].id]
			if prunedNode != nil {
				prunedNode.RemoveNeighbor(layer, node.ID)
			}
		}
	}
}

// GetVector returns the vector for the given ID.
func (idx *Index) GetVector(id uint64) ([]float32, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	v, ok := idx.getVector(id)
	if !ok {
		return nil, false
	}
	result := make([]float32, len(v))
	copy(result, v)
	return result, true
}

// HasNode returns true if a node with the given ID exists.
func (idx *Index) HasNode(id uint64) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	_, ok := idx.nodes[id]
	return ok
}

// Stats returns statistics about the index.
func (idx *Index) Stats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	stats := IndexStats{
		NodeCount:  len(idx.nodes),
		MaxLevel:   idx.maxLevel,
		Dimension:  idx.dimension,
		EntryPoint: idx.entryPoint,
		Config:     idx.config,
	}

	// Count nodes at each level
	stats.LevelCounts = make([]int, idx.maxLevel+1)
	deletedCount := 0
	totalConnections := 0

	for _, node := range idx.nodes {
		if node.Deleted {
			deletedCount++
		}
		for l := 0; l <= node.Level; l++ {
			stats.LevelCounts[l]++
			totalConnections += len(node.Neighbors[l])
		}
	}

	stats.DeletedCount = deletedCount
	if len(idx.nodes) > 0 {
		stats.AvgConnections = float64(totalConnections) / float64(len(idx.nodes))
	}

	return stats
}

// IndexStats contains statistics about an HNSW index.
type IndexStats struct {
	NodeCount      int
	DeletedCount   int
	MaxLevel       int
	Dimension      int
	EntryPoint     uint64
	LevelCounts    []int
	AvgConnections float64
	Config         Config
}
