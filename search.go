package veclite

// SearchOption configures search behavior.
type SearchOption interface {
	apply(*searchConfig)
}

// searchConfig holds the search configuration.
type searchConfig struct {
	topK      int
	threshold *float32
	filters   []Filter
}

// defaultSearchConfig returns the default search configuration.
func defaultSearchConfig() *searchConfig {
	return &searchConfig{
		topK:    10,
		filters: nil,
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
