package veclite

// SearchOption configures search behavior.
type SearchOption interface {
	apply(*searchConfig)
}

// searchConfig holds the search configuration.
type searchConfig struct {
	topK            int
	threshold       *float32
	filters         []Filter
	efSearch        int
	offset          int
	limit           int
	includeContent  bool
	vectorWeight    float64
	textWeight      float64
	decay           *DecayConfig
	importanceBoost float32
	accessTracking  bool
}

// defaultSearchConfig returns the default search configuration.
func defaultSearchConfig() *searchConfig {
	return &searchConfig{
		topK:     10,
		filters:  nil,
		efSearch: 0, // 0 means use index default
	}
}

// searchOptionFunc is a function adapter for SearchOption.
type searchOptionFunc func(*searchConfig)

func (f searchOptionFunc) apply(c *searchConfig) {
	f(c)
}

// TopK sets the maximum number of results to return.
// Default is 10.
func TopK(k int) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		if k > 0 {
			c.topK = k
		}
	})
}

// Threshold sets the minimum similarity score for results.
// For cosine/dot: results with score >= threshold are returned.
// For euclidean: results with score <= threshold are returned.
func Threshold(t float32) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		c.threshold = &t
	})
}

// WithFilter adds a filter to the search.
// Multiple filters are combined with AND logic.
func WithFilter(f Filter) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		c.filters = append(c.filters, f)
	})
}

// WithFilters adds multiple filters to the search.
// All filters are combined with AND logic.
func WithFilters(filters ...Filter) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		c.filters = append(c.filters, filters...)
	})
}

// applyFilters checks if a record matches all filters.
func (c *searchConfig) matchesFilters(r *Record) bool {
	for _, f := range c.filters {
		if !f.Match(r) {
			return false
		}
	}
	return true
}

// WithEfSearch sets the efSearch parameter for HNSW search.
// Higher values improve recall at the cost of speed.
// Has no effect on collections without HNSW index.
func WithEfSearch(ef int) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		if ef > 0 {
			c.efSearch = ef
		}
	})
}

// WithOffset sets the number of results to skip before returning.
// Use with TopK for pagination: WithOffset(20), TopK(10) returns results 21-30.
func WithOffset(n int) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		if n >= 0 {
			c.offset = n
		}
	})
}

// WithLimit sets the maximum number of results to return.
// This is an alias for TopK for use in pagination contexts.
func WithLimit(n int) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		if n > 0 {
			c.limit = n
		}
	})
}

// WithContent controls whether the Content field is included in search results.
// By default, Content is included. Set to false to exclude it for smaller results.
func WithContent(include bool) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		c.includeContent = include
	})
}

// WithVectorWeight sets the weight for the vector search component in hybrid search.
// Default is 1.0.
func WithVectorWeight(w float64) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		c.vectorWeight = w
	})
}

// WithTextWeight sets the weight for the text search component in hybrid search.
// Default is 1.0.
func WithTextWeight(w float64) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		c.textWeight = w
	})
}

// effectiveTopK returns the number of results to fetch internally,
// accounting for offset when pagination is used.
func (c *searchConfig) effectiveTopK() int {
	if c.offset > 0 {
		return c.topK + c.offset
	}
	return c.topK
}

// applyPagination applies offset to results and limits to topK.
func (c *searchConfig) applyPagination(results []Result) []Result {
	if c.offset > 0 {
		if c.offset >= len(results) {
			return nil
		}
		results = results[c.offset:]
	}
	limit := c.topK
	if c.limit > 0 {
		limit = c.limit
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}
