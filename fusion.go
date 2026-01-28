package veclite

import "sort"

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
