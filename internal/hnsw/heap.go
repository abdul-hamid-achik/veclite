package hnsw

import "container/heap"

// Item represents an item in the priority queue.
type Item struct {
	ID       uint64
	Distance float32
}

// MaxHeap is a max-heap of items (largest distance at top).
// Used for maintaining the k nearest neighbors during search.
type MaxHeap []Item

func (h MaxHeap) Len() int           { return len(h) }
func (h MaxHeap) Less(i, j int) bool { return h[i].Distance > h[j].Distance } // Max heap
func (h MaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MaxHeap) Push(x any) {
	*h = append(*h, x.(Item))
}

func (h *MaxHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// Peek returns the top item without removing it.
func (h MaxHeap) Peek() Item {
	return h[0]
}

// MinHeap is a min-heap of items (smallest distance at top).
// Used for greedy search traversal.
type MinHeap []Item

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i].Distance < h[j].Distance } // Min heap
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x any) {
	*h = append(*h, x.(Item))
}

func (h *MinHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// Peek returns the top item without removing it.
func (h MinHeap) Peek() Item {
	return h[0]
}

// CandidateSet is a set of candidates used during HNSW search.
// It maintains a heap for traversal and tracks the best results.
type CandidateSet struct {
	candidates   MinHeap // For lowerBetter traversal
	candidatesHB MaxHeap // For higherBetter traversal
	results      []Item  // Best results found
	maxSize      int
	visited      map[uint64]bool
	higherBetter bool
}

// NewCandidateSet creates a new candidate set with the given maximum size.
// higherBetter indicates if higher distance values are better (true for cosine/dot).
func NewCandidateSet(maxSize int, higherBetter bool) *CandidateSet {
	cs := &CandidateSet{
		results:      make([]Item, 0, maxSize),
		maxSize:      maxSize,
		visited:      make(map[uint64]bool),
		higherBetter: higherBetter,
	}
	if higherBetter {
		cs.candidatesHB = make(MaxHeap, 0, maxSize)
	} else {
		cs.candidates = make(MinHeap, 0, maxSize)
	}
	return cs
}

// Add adds a candidate if not already visited.
// Returns true if added, false if already visited.
func (cs *CandidateSet) Add(id uint64, distance float32) bool {
	if cs.visited[id] {
		return false
	}
	cs.visited[id] = true

	item := Item{ID: id, Distance: distance}

	// Add to appropriate candidate heap
	if cs.higherBetter {
		heap.Push(&cs.candidatesHB, item)
	} else {
		heap.Push(&cs.candidates, item)
	}

	// Maintain results list
	cs.addToResults(item)
	return true
}

// addToResults adds an item to results, maintaining best K items.
func (cs *CandidateSet) addToResults(item Item) {
	if len(cs.results) < cs.maxSize {
		cs.results = append(cs.results, item)
		return
	}

	// Find and potentially replace worst result
	worstIdx := 0
	for i := 1; i < len(cs.results); i++ {
		if cs.isBetter(cs.results[worstIdx], cs.results[i]) {
			worstIdx = i
		}
	}

	if cs.isBetter(item, cs.results[worstIdx]) {
		cs.results[worstIdx] = item
	}
}

// isBetter returns true if a is better than b.
func (cs *CandidateSet) isBetter(a, b Item) bool {
	if cs.higherBetter {
		return a.Distance > b.Distance
	}
	return a.Distance < b.Distance
}

// HasCandidates returns true if there are candidates to process.
func (cs *CandidateSet) HasCandidates() bool {
	if cs.higherBetter {
		return cs.candidatesHB.Len() > 0
	}
	return cs.candidates.Len() > 0
}

// PopNearest removes and returns the most promising unprocessed candidate.
func (cs *CandidateSet) PopNearest() Item {
	if cs.higherBetter {
		return heap.Pop(&cs.candidatesHB).(Item)
	}
	return heap.Pop(&cs.candidates).(Item)
}

// WorstDistance returns the distance of the worst result.
func (cs *CandidateSet) WorstDistance() float32 {
	if len(cs.results) == 0 {
		if cs.higherBetter {
			return float32(-1e38)
		}
		return float32(1e38)
	}

	worst := cs.results[0].Distance
	for i := 1; i < len(cs.results); i++ {
		if cs.higherBetter {
			if cs.results[i].Distance < worst {
				worst = cs.results[i].Distance
			}
		} else {
			if cs.results[i].Distance > worst {
				worst = cs.results[i].Distance
			}
		}
	}
	return worst
}

// ResultsFull returns true if results are at capacity.
func (cs *CandidateSet) ResultsFull() bool {
	return len(cs.results) >= cs.maxSize
}

// Results returns the current results as a sorted slice (best first).
func (cs *CandidateSet) Results() []Item {
	result := make([]Item, len(cs.results))
	copy(result, cs.results)

	// Sort by distance (best first)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if cs.higherBetter {
				// Higher is better - sort descending
				if result[i].Distance < result[j].Distance {
					result[i], result[j] = result[j], result[i]
				}
			} else {
				// Lower is better - sort ascending
				if result[i].Distance > result[j].Distance {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
	}
	return result
}

// IsVisited returns true if the ID has been visited.
func (cs *CandidateSet) IsVisited(id uint64) bool {
	return cs.visited[id]
}
