package hnsw

import (
	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// Snapshot is a serializable representation of an HNSW index.
type Snapshot struct {
	Config       Config
	EntryPoint   uint64
	MaxLevel     int
	Dimension    int
	DistanceType floats.DistanceType
	Nodes        []NodeSnapshot
	Vectors      map[uint64][]float32
}

// NodeSnapshot is a serializable representation of a node.
type NodeSnapshot struct {
	ID        uint64
	Level     int
	Neighbors [][]uint64
	Deleted   bool
}

// Snapshot creates a serializable snapshot of the index.
func (idx *Index) Snapshot() *Snapshot {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	snap := &Snapshot{
		Config:     idx.config,
		EntryPoint: idx.entryPoint,
		MaxLevel:   idx.maxLevel,
		Dimension:  idx.dimension,
		Nodes:      make([]NodeSnapshot, 0, len(idx.nodes)),
		Vectors:    make(map[uint64][]float32, len(idx.vectors)),
	}

	// Snapshot nodes
	for _, node := range idx.nodes {
		neighbors := make([][]uint64, len(node.Neighbors))
		for i, layer := range node.Neighbors {
			neighbors[i] = make([]uint64, len(layer))
			copy(neighbors[i], layer)
		}

		snap.Nodes = append(snap.Nodes, NodeSnapshot{
			ID:        node.ID,
			Level:     node.Level,
			Neighbors: neighbors,
			Deleted:   node.Deleted,
		})
	}

	// Snapshot vectors
	for id, vec := range idx.vectors {
		v := make([]float32, len(vec))
		copy(v, vec)
		snap.Vectors[id] = v
	}

	return snap
}

// LoadFromSnapshot restores an index from a snapshot.
func LoadFromSnapshot(snap *Snapshot, distanceType floats.DistanceType) *Index {
	idx := &Index{
		config:       snap.Config,
		nodes:        make(map[uint64]*Node, len(snap.Nodes)),
		vectors:      make(map[uint64][]float32, len(snap.Vectors)),
		entryPoint:   snap.EntryPoint,
		maxLevel:     snap.MaxLevel,
		dimension:    snap.Dimension,
		distFunc:     floats.GetDistanceFunc(distanceType),
		higherBetter: floats.IsHigherBetter(distanceType),
	}

	// Restore nodes
	for _, ns := range snap.Nodes {
		node := &Node{
			ID:        ns.ID,
			Level:     ns.Level,
			Neighbors: make([][]uint64, len(ns.Neighbors)),
			Deleted:   ns.Deleted,
		}
		for i, layer := range ns.Neighbors {
			node.Neighbors[i] = make([]uint64, len(layer))
			copy(node.Neighbors[i], layer)
		}
		idx.nodes[ns.ID] = node
	}

	// Restore vectors
	for id, vec := range snap.Vectors {
		v := make([]float32, len(vec))
		copy(v, vec)
		idx.vectors[id] = v
	}

	return idx
}

// Clone creates a deep copy of the index.
func (idx *Index) Clone() *Index {
	snap := idx.Snapshot()
	// Use a default distance type; the actual type is determined by the collection
	return LoadFromSnapshot(snap, floats.DistanceCosine)
}
