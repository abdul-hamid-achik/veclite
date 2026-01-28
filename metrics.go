package veclite

import (
	"sync/atomic"
	"time"
)

// Metrics provides observable counters for database operations.
// All operations are atomic and safe for concurrent access.
type Metrics struct {
	searchCount  atomic.Int64
	insertCount  atomic.Int64
	deleteCount  atomic.Int64
	searchTimeNs atomic.Int64 // cumulative search time in nanoseconds
	searchCount2 atomic.Int64 // count for latency calculation (same as searchCount but avoids race)
}

// MetricsSnapshot is a point-in-time snapshot of database metrics.
type MetricsSnapshot struct {
	SearchCount    int64         `json:"search_count"`
	InsertCount    int64         `json:"insert_count"`
	DeleteCount    int64         `json:"delete_count"`
	AvgSearchTime  time.Duration `json:"avg_search_time_ns"`
}

func newMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) recordSearch(d time.Duration) {
	m.searchCount.Add(1)
	m.searchTimeNs.Add(int64(d))
	m.searchCount2.Add(1)
}

func (m *Metrics) recordInsert() {
	m.insertCount.Add(1)
}

func (m *Metrics) recordDelete() {
	m.deleteCount.Add(1)
}

// Snapshot returns a point-in-time snapshot of the metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	sc := m.searchCount2.Load()
	totalNs := m.searchTimeNs.Load()
	var avg time.Duration
	if sc > 0 {
		avg = time.Duration(totalNs / sc)
	}

	return MetricsSnapshot{
		SearchCount:   m.searchCount.Load(),
		InsertCount:   m.insertCount.Load(),
		DeleteCount:   m.deleteCount.Load(),
		AvgSearchTime: avg,
	}
}
