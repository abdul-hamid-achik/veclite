package veclite

import "sort"

// FuseOption configures result fusion (see FuseRRF).
type FuseOption interface {
	apply(*fuseConfig)
}

type fuseConfig struct {
	k       int
	weights []float64
	topK    int
}

type fuseOptionFunc func(*fuseConfig)

func (f fuseOptionFunc) apply(c *fuseConfig) { f(c) }

// WithRRFK sets the Reciprocal Rank Fusion constant k (default 60). Higher
// values flatten the influence of top ranks.
func WithRRFK(k int) FuseOption {
	return fuseOptionFunc(func(c *fuseConfig) {
		if k > 0 {
			c.k = k
		}
	})
}

// WithFusionWeights sets a per-result-set weight. The i-th weight scales the
// contribution of the i-th result set. Missing weights default to 1.0.
func WithFusionWeights(weights ...float64) FuseOption {
	return fuseOptionFunc(func(c *fuseConfig) {
		c.weights = weights
	})
}

// WithFusionTopK truncates the fused output to at most n results (0 = no limit).
func WithFusionTopK(n int) FuseOption {
	return fuseOptionFunc(func(c *fuseConfig) {
		if n > 0 {
			c.topK = n
		}
	})
}

// FuseRRF merges multiple ranked result sets into a single ranking using
// Reciprocal Rank Fusion. It is the public, modality-agnostic fusion primitive
// behind HybridSearch and Collection.MultiSpaceSearch: callers can fuse any mix
// of vector-space results, BM25 text results, or externally produced rankings.
//
// Records are deduplicated by ID; a record appearing in several sets accumulates
// the (weighted) reciprocal-rank contribution of each. The returned results are
// sorted by fused score descending.
func FuseRRF(resultSets [][]Result, opts ...FuseOption) []Result {
	cfg := &fuseConfig{k: 60}
	for _, opt := range opts {
		opt.apply(cfg)
	}

	fused := reciprocalRankFusion(resultSets, cfg.k, cfg.weights)
	if cfg.topK > 0 && len(fused) > cfg.topK {
		fused = fused[:cfg.topK]
	}
	return fused
}

// reciprocalRankFusion merges multiple ranked result lists using RRF.
// k is the RRF constant (typically 60). Higher k values reduce the impact of high rankings.
// weights allows weighting each result set (pass nil for equal weights).
func reciprocalRankFusion(resultSets [][]Result, k int, weights []float64) []Result {
	if k <= 0 {
		k = 60
	}

	scores := make(map[uint64]float64)
	records := make(map[uint64]*Record)

	for setIdx, results := range resultSets {
		w := 1.0
		if weights != nil && setIdx < len(weights) {
			w = weights[setIdx]
		}

		for rank, result := range results {
			id := result.Record.ID
			scores[id] += w / float64(k+rank+1)
			// Keep the record from whichever set provides it first
			if _, exists := records[id]; !exists {
				records[id] = result.Record
			}
		}
	}

	fused := make([]Result, 0, len(scores))
	for id, score := range scores {
		fused = append(fused, Result{
			Record: records[id],
			Score:  float32(score),
		})
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})

	return fused
}
