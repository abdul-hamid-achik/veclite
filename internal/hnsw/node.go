package hnsw

// Node represents a node in the HNSW graph.
type Node struct {
	// ID is the unique identifier for this node.
	ID uint64

	// Level is the maximum layer this node appears in (0-indexed).
	// A node at level L appears in layers 0, 1, ..., L.
	Level int

	// Neighbors contains the neighbor lists for each layer.
	// neighbors[layer] is a slice of neighbor IDs at that layer.
	Neighbors [][]uint64

	// Deleted marks this node as deleted (soft delete).
	// Deleted nodes are skipped during search but remain in the graph.
	Deleted bool
}

// NewNode creates a new node with the given ID and level.
func NewNode(id uint64, level int) *Node {
	n := &Node{
		ID:        id,
		Level:     level,
		Neighbors: make([][]uint64, level+1),
	}
	for i := 0; i <= level; i++ {
		n.Neighbors[i] = make([]uint64, 0)
	}
	return n
}

// GetNeighbors returns the neighbors at the given layer.
// Returns nil if layer is out of range.
func (n *Node) GetNeighbors(layer int) []uint64 {
	if layer < 0 || layer > n.Level {
		return nil
	}
	return n.Neighbors[layer]
}

// SetNeighbors sets the neighbors at the given layer.
func (n *Node) SetNeighbors(layer int, neighbors []uint64) {
	if layer >= 0 && layer <= n.Level {
		n.Neighbors[layer] = neighbors
	}
}

// AddNeighbor adds a neighbor at the given layer if not already present.
func (n *Node) AddNeighbor(layer int, neighborID uint64) {
	if layer < 0 || layer > n.Level {
		return
	}
	if n.HasNeighbor(layer, neighborID) {
		return
	}
	n.Neighbors[layer] = append(n.Neighbors[layer], neighborID)
}

// HasNeighbor returns true if the node has the given neighbor at the layer.
func (n *Node) HasNeighbor(layer int, neighborID uint64) bool {
	if layer < 0 || layer > n.Level {
		return false
	}
	for _, id := range n.Neighbors[layer] {
		if id == neighborID {
			return true
		}
	}
	return false
}

// RemoveNeighbor removes a neighbor at the given layer.
func (n *Node) RemoveNeighbor(layer int, neighborID uint64) {
	if layer < 0 || layer > n.Level {
		return
	}
	neighbors := n.Neighbors[layer]
	for i, id := range neighbors {
		if id == neighborID {
			n.Neighbors[layer] = append(neighbors[:i], neighbors[i+1:]...)
			return
		}
	}
}

// NeighborCount returns the number of neighbors at the given layer.
func (n *Node) NeighborCount(layer int) int {
	if layer < 0 || layer > n.Level {
		return 0
	}
	return len(n.Neighbors[layer])
}

// TotalNeighbors returns the total number of neighbors across all layers.
func (n *Node) TotalNeighbors() int {
	total := 0
	for _, neighbors := range n.Neighbors {
		total += len(neighbors)
	}
	return total
}
