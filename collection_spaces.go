package veclite

import (
	"fmt"
	"sort"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// cloneProfile returns a shallow copy of an embedding profile (all fields are
// value types), or nil.
func cloneProfile(p *EmbeddingProfile) *EmbeddingProfile {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// AddVectorSpace declares an additional named vector space on the collection.
//
// The space is independent of the default space and of other spaces: it has its
// own dimension, distance metric, and optional HNSW index. After declaring it,
// insert vectors into it with InsertRecord (or SetRecordVector) and query it
// with SearchSpace. The reserved name DefaultVectorSpace cannot be used.
func (c *Collection) AddVectorSpace(config VectorSpaceConfig) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()
	err := c.addVectorSpaceLocked(config)
	c.mu.Unlock()
	if err != nil {
		return err
	}

	c.syncIfNeeded()
	return nil
}

// addVectorSpaceLocked declares a vector space. The caller must hold c.mu for
// writing, or guarantee exclusive access (as during construction).
func (c *Collection) addVectorSpaceLocked(config VectorSpaceConfig) error {
	if config.Name == "" || config.Name == DefaultVectorSpace {
		return fmt.Errorf("%w: name %q is empty or reserved", ErrInvalidVectorSpace, config.Name)
	}
	if c.spaces == nil {
		c.spaces = make(map[string]*vectorSpace)
	}
	if _, ok := c.spaces[config.Name]; ok {
		return fmt.Errorf("%w: %q", ErrVectorSpaceExists, config.Name)
	}

	distance := config.Distance
	if distance == "" {
		distance = floats.DistanceCosine
	}

	dimension := config.Dimension
	profile := cloneProfile(config.Profile)
	if profile != nil && dimension == 0 && profile.Dimension > 0 {
		dimension = profile.Dimension
	}

	sp := &vectorSpace{
		name:         config.Name,
		dimension:    dimension,
		distanceType: distance,
		distanceFunc: floats.GetDistanceFunc(distance),
		higherBetter: floats.IsHigherBetter(distance),
		modality:     config.Modality,
		provider:     config.Provider,
		model:        config.Model,
		indexType:    IndexTypeNone,
		profile:      profile,
	}

	if config.HNSW != nil {
		sp.indexType = IndexTypeHNSW
		hc := *config.HNSW
		sp.hnswConfig = &hc
		c.initSpaceIndexIfNeeded(sp)
	}

	c.spaces[config.Name] = sp
	return nil
}

// initSpaceIndexIfNeeded builds the space's HNSW index once its dimension is
// known. The caller must hold c.mu for writing.
func (c *Collection) initSpaceIndexIfNeeded(sp *vectorSpace) {
	if sp.indexType == IndexTypeHNSW && sp.hnswConfig != nil && sp.index == nil && sp.dimension > 0 {
		sp.index = newHNSWIndex(sp.dimension, sp.distanceType, sp.hnswConfig)
		c.setupSpaceVectorProvider(sp)
	}
}

// setupSpaceVectorProvider points the space's HNSW index at the records' vectors
// for that space, avoiding duplicate vector storage.
func (c *Collection) setupSpaceVectorProvider(sp *vectorSpace) {
	if sp.index == nil {
		return
	}
	hnswIdx, ok := sp.index.(*hnswIndex)
	if !ok {
		return
	}
	name := sp.name
	hnswIdx.internal().SetVectorProvider(func(id uint64) ([]float32, bool) {
		rec, ok := c.records[id]
		if !ok {
			return nil, false
		}
		vec, ok := rec.Vectors[name]
		if !ok || len(vec) == 0 {
			return nil, false
		}
		return vec, true
	})
}

// removeFromSpaceIndexesLocked removes a record's vectors from every named-space
// index it participates in. The caller must hold c.mu for writing. Call this
// whenever a record is deleted or converted to text-only so the per-space HNSW
// indexes do not leak nodes (a space index reads vectors from c.records via its
// vector provider, so a dangling node would otherwise persist after the record
// is gone — and could corrupt search if it were the graph entry point).
func (c *Collection) removeFromSpaceIndexesLocked(id uint64, record *Record) {
	if record == nil || len(record.Vectors) == 0 || len(c.spaces) == 0 {
		return
	}
	for name, sp := range c.spaces {
		if sp.index == nil {
			continue
		}
		if vec, ok := record.Vectors[name]; ok && len(vec) > 0 {
			c.hardDeleteFromSpaceIndex(sp, id)
		}
	}
}

// hardDeleteFromSpaceIndex removes a vector from a space's index completely,
// needed before re-inserting under the same ID.
func (c *Collection) hardDeleteFromSpaceIndex(sp *vectorSpace, id uint64) {
	if sp.index == nil {
		return
	}
	if hnswIdx, ok := sp.index.(*hnswIndex); ok {
		_ = hnswIdx.hardDelete(id)
		return
	}
	_ = sp.index.Delete(id)
}

// HasVectorSpace reports whether the collection has the named space. The default
// space always exists.
func (c *Collection) HasVectorSpace(name string) bool {
	if name == "" || name == DefaultVectorSpace {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.spaces[name]
	return ok
}

// VectorSpaces returns information about every vector space on the collection,
// starting with the default space, followed by named spaces in name order.
func (c *Collection) VectorSpaces() []VectorSpaceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	infos := make([]VectorSpaceInfo, 0, len(c.spaces)+1)

	defaultCount := 0
	for _, r := range c.records {
		if len(r.Vector) > 0 {
			defaultCount++
		}
	}
	defaultIndex := string(c.indexType)
	if defaultIndex == "" {
		defaultIndex = string(IndexTypeNone)
	}
	infos = append(infos, VectorSpaceInfo{
		Name:        DefaultVectorSpace,
		Dimension:   c.dimension,
		Distance:    c.distanceType,
		IndexType:   defaultIndex,
		VectorCount: defaultCount,
		Profile:     cloneProfile(c.profile),
	})

	names := make([]string, 0, len(c.spaces))
	for name := range c.spaces {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		sp := c.spaces[name]
		count := 0
		for _, r := range c.records {
			if vec, ok := r.Vectors[name]; ok && len(vec) > 0 {
				count++
			}
		}
		indexType := string(sp.indexType)
		if indexType == "" {
			indexType = string(IndexTypeNone)
		}
		infos = append(infos, VectorSpaceInfo{
			Name:        sp.name,
			Dimension:   sp.dimension,
			Distance:    sp.distanceType,
			Modality:    sp.modality,
			Provider:    sp.provider,
			Model:       sp.model,
			IndexType:   indexType,
			VectorCount: count,
			Profile:     cloneProfile(sp.profile),
		})
	}

	return infos
}

// VectorSpace returns information about a single vector space, or
// ErrVectorSpaceNotFound.
func (c *Collection) VectorSpace(name string) (VectorSpaceInfo, error) {
	if name == "" {
		name = DefaultVectorSpace
	}
	for _, info := range c.VectorSpaces() {
		if info.Name == name {
			return info, nil
		}
	}
	return VectorSpaceInfo{}, fmt.Errorf("%w: %q", ErrVectorSpaceNotFound, name)
}

// SetEmbeddingProfile attaches (or replaces) the collection's first-class
// default-space embedding profile. Subsequent default-space inserts validate
// against the profile's dimension.
func (c *Collection) SetEmbeddingProfile(profile EmbeddingProfile) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}
	c.mu.Lock()
	// Keep the default-space dimension consistent with the profile so that every
	// insert path (Insert, InsertWithOptions, Upsert, UpdateVector, InsertDocument,
	// InsertRecord) enforces it via the collection dimension check — not only
	// InsertRecord. Reject a profile whose dimension conflicts with an established one.
	if profile.Dimension > 0 {
		if c.dimension == 0 {
			c.dimension = profile.Dimension
			c.initHNSWIfNeeded()
		} else if c.dimension != profile.Dimension {
			c.mu.Unlock()
			return fmt.Errorf("%w: profile dimension %d does not match collection dimension %d",
				ErrProfileMismatch, profile.Dimension, c.dimension)
		}
	}
	p := profile
	c.profile = &p
	c.mu.Unlock()
	c.syncIfNeeded()
	return nil
}

// EmbeddingProfile returns the collection's default-space embedding profile and
// whether one is set.
func (c *Collection) EmbeddingProfile() (EmbeddingProfile, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.profile == nil {
		return EmbeddingProfile{}, false
	}
	return *c.profile, true
}

// InsertRecord inserts (or, when in.ID names an existing record, replaces) one
// logical record that may carry vectors in several named vector spaces at once,
// plus optional content and payload. Returns the record ID.
//
// Each key of in.Vectors must be DefaultVectorSpace (or "") or a space declared
// via AddVectorSpace; unknown spaces return ErrVectorSpaceNotFound. Vectors are
// validated against each space's dimension and embedding profile before any
// state changes.
func (c *Collection) InsertRecord(in RecordInput) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}

	id, err := c.insertRecordLocked(in)
	if err != nil {
		return 0, err
	}

	if c.db != nil && c.db.metrics != nil {
		c.db.metrics.recordInsert()
	}
	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()

	if record, err := c.Get(id); err == nil {
		c.notifySubscribers(record)
	}

	return id, nil
}

func (c *Collection) insertRecordLocked(in RecordInput) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Validate and split the supplied vectors by space.
	var defaultVec []float32
	named := make(map[string][]float32)
	for name, vec := range in.Vectors {
		if len(vec) == 0 {
			continue
		}
		if name == "" || name == DefaultVectorSpace {
			if c.dimension != 0 && len(vec) != c.dimension {
				return 0, &DimensionError{Expected: c.dimension, Got: len(vec)}
			}
			if c.profile != nil {
				if err := c.profile.validateVector(vec); err != nil {
					return 0, err
				}
			}
			defaultVec = vec
			continue
		}
		sp, ok := c.spaces[name]
		if !ok {
			return 0, fmt.Errorf("%w: %q", ErrVectorSpaceNotFound, name)
		}
		if sp.dimension != 0 && len(vec) != sp.dimension {
			return 0, &DimensionError{Expected: sp.dimension, Got: len(vec)}
		}
		if sp.profile != nil {
			if err := sp.profile.validateVector(vec); err != nil {
				return 0, err
			}
		}
		named[name] = vec
	}

	// 2. Resolve the record ID (replace when it already exists).
	now := time.Now()
	id := in.ID
	var old *Record
	if id != 0 {
		old = c.records[id]
	}
	if id == 0 {
		id = c.nextID
		c.nextID++
	} else if id >= c.nextID {
		c.nextID = id + 1
	}

	// Remove the old record's vectors from every index before re-inserting.
	if old != nil {
		if c.index != nil && len(old.Vector) > 0 {
			c.hardDeleteFromIndex(id)
		}
		for name, sp := range c.spaces {
			if sp.index != nil {
				if vec, ok := old.Vectors[name]; ok && len(vec) > 0 {
					c.hardDeleteFromSpaceIndex(sp, id)
				}
			}
		}
	}

	// 3. Build the record, preserving prior bookkeeping on replace.
	rec := &Record{
		ID:        id,
		Content:   in.Content,
		Payload:   in.Payload,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if old != nil {
		rec.CreatedAt = old.CreatedAt
		rec.ExpiresAt = old.ExpiresAt
		rec.Importance = old.Importance
		rec.AccessCount = old.AccessCount
		rec.LastAccessedAt = old.LastAccessedAt
	}

	if len(defaultVec) > 0 {
		if c.dimension == 0 {
			c.dimension = len(defaultVec)
			c.initHNSWIfNeeded()
		}
		rec.Vector = make([]float32, len(defaultVec))
		copy(rec.Vector, defaultVec)
	}

	if len(named) > 0 {
		rec.Vectors = make(map[string][]float32, len(named))
		for name, vec := range named {
			sp := c.spaces[name]
			if sp.dimension == 0 {
				sp.dimension = len(vec)
				c.initSpaceIndexIfNeeded(sp)
			}
			cp := make([]float32, len(vec))
			copy(cp, vec)
			rec.Vectors[name] = cp
		}
	}

	c.records[id] = rec

	// 4. Insert into each index, rolling back on failure.
	inserted := make([]*vectorSpace, 0, len(named))
	if c.index != nil && len(rec.Vector) > 0 {
		if err := c.index.Insert(id, rec.Vector); err != nil {
			delete(c.records, id)
			return 0, err
		}
	}
	for name, vec := range rec.Vectors {
		sp := c.spaces[name]
		if sp.index == nil {
			continue
		}
		if err := sp.index.Insert(id, vec); err != nil {
			// Best-effort rollback of indexes already touched for this record.
			if c.index != nil && len(rec.Vector) > 0 {
				c.hardDeleteFromIndex(id)
			}
			for _, done := range inserted {
				c.hardDeleteFromSpaceIndex(done, id)
			}
			delete(c.records, id)
			return 0, err
		}
		inserted = append(inserted, sp)
	}

	// 5. (Re)index text content.
	c.reindexRecordLocked(rec)

	return id, nil
}

// SetRecordVector sets or replaces a single space's vector on an existing
// record, leaving the record's other spaces, content, and payload untouched.
// Use DefaultVectorSpace (or "") to target the default space.
func (c *Collection) SetRecordVector(id uint64, space string, vector []float32) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}
	if len(vector) == 0 {
		return ErrEmptyVector
	}
	if space == "" || space == DefaultVectorSpace {
		return c.UpdateVector(id, vector)
	}

	c.mu.Lock()

	rec, ok := c.records[id]
	if !ok {
		c.mu.Unlock()
		return &NotFoundError{Type: "record", ID: fmt.Sprintf("%d", id)}
	}
	sp, ok := c.spaces[space]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("%w: %q", ErrVectorSpaceNotFound, space)
	}
	if sp.dimension == 0 {
		sp.dimension = len(vector)
		c.initSpaceIndexIfNeeded(sp)
	} else if len(vector) != sp.dimension {
		c.mu.Unlock()
		return &DimensionError{Expected: sp.dimension, Got: len(vector)}
	}
	if sp.profile != nil {
		if err := sp.profile.validateVector(vector); err != nil {
			c.mu.Unlock()
			return err
		}
	}

	if rec.Vectors == nil {
		rec.Vectors = make(map[string][]float32)
	}
	if sp.index != nil {
		if old, ok := rec.Vectors[space]; ok && len(old) > 0 {
			c.hardDeleteFromSpaceIndex(sp, id)
		}
		if err := sp.index.Insert(id, vector); err != nil {
			c.mu.Unlock()
			return err
		}
	}
	cp := make([]float32, len(vector))
	copy(cp, vector)
	rec.Vectors[space] = cp
	rec.UpdatedAt = time.Now()
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// SearchSpace searches a single named vector space. Passing DefaultVectorSpace
// (or "") is equivalent to Search. All standard SearchOptions apply (TopK,
// filters, threshold, pagination, efSearch).
func (c *Collection) SearchSpace(space string, query []float32, opts ...SearchOption) ([]Result, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}
	if space == "" || space == DefaultVectorSpace {
		return c.Search(query, opts...)
	}

	config := defaultSearchConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	c.mu.RLock()
	sp, ok := c.spaces[space]
	if !ok {
		c.mu.RUnlock()
		return nil, fmt.Errorf("%w: %q", ErrVectorSpaceNotFound, space)
	}
	if sp.dimension > 0 && len(query) != sp.dimension {
		c.mu.RUnlock()
		return nil, &DimensionError{Expected: sp.dimension, Got: len(query)}
	}
	results, err := c.searchSpaceLocked(sp, query, config)
	c.mu.RUnlock()
	if err != nil {
		return nil, err
	}

	if config.accessTracking && len(results) > 0 {
		c.trackAccess(results)
	}
	return config.applyPagination(results), nil
}

// searchSpaceLocked runs a search over one space. Caller holds c.mu for reading.
func (c *Collection) searchSpaceLocked(sp *vectorSpace, query []float32, config *searchConfig) ([]Result, error) {
	effectiveK := config.effectiveTopK()
	if effectiveK <= 0 {
		effectiveK = 10
	}

	if sp.index != nil && sp.index.Count() > 0 {
		ef := config.efSearch
		if ef == 0 && sp.hnswConfig != nil {
			ef = sp.hnswConfig.EfSearch
		}
		requestK := effectiveK
		if len(config.filters) > 0 {
			requestK = max(effectiveK*8, 100)
		}

		var indexResults []IndexResult
		var err error
		if ef > 0 {
			indexResults, err = sp.index.SearchWithEf(query, requestK, ef)
		} else {
			indexResults, err = sp.index.Search(query, requestK)
		}
		if err != nil {
			return nil, err
		}

		results := make([]Result, 0, len(indexResults))
		for _, ir := range indexResults {
			rec, ok := c.records[ir.ID]
			if !ok {
				continue
			}
			if !config.matchesFilters(rec) {
				continue
			}
			if config.threshold != nil {
				if sp.higherBetter && ir.Distance < *config.threshold {
					continue
				}
				if !sp.higherBetter && ir.Distance > *config.threshold {
					continue
				}
			}
			results = append(results, Result{
				Record: config.cloneRecordForResult(rec),
				Score:  ir.Distance,
			})
			if len(results) >= effectiveK {
				break
			}
		}

		// With enough results, or no filters to satisfy, return as-is. Otherwise
		// fall through to a brute-force pass for completeness.
		if len(results) >= effectiveK || len(config.filters) == 0 {
			return results, nil
		}
	}

	return c.searchSpaceBruteForce(sp, query, config, effectiveK), nil
}

func (c *Collection) searchSpaceBruteForce(sp *vectorSpace, query []float32, config *searchConfig, effectiveK int) []Result {
	results := make([]Result, 0)
	for _, rec := range c.records {
		vec, ok := rec.Vectors[sp.name]
		if !ok || len(vec) == 0 {
			continue
		}
		if !config.matchesFilters(rec) {
			continue
		}
		score := sp.distanceFunc(query, vec)
		if config.threshold != nil {
			if sp.higherBetter && score < *config.threshold {
				continue
			}
			if !sp.higherBetter && score > *config.threshold {
				continue
			}
		}
		results = append(results, Result{
			Record: config.cloneRecordForResult(rec),
			Score:  score,
		})
	}

	sortResultsByScore(results, sp.higherBetter)
	if len(results) > effectiveK {
		results = results[:effectiveK]
	}
	return results
}

// sortResultsByScore sorts results by score with a stable record-ID tiebreaker
// so pagination is deterministic.
func sortResultsByScore(results []Result, higherBetter bool) {
	if higherBetter {
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score > results[j].Score
			}
			return results[i].Record.ID < results[j].Record.ID
		})
		return
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score < results[j].Score
		}
		return results[i].Record.ID < results[j].Record.ID
	})
}

// MultiSpaceSearch runs one query per named vector space and fuses the result
// sets into a single ranking with Reciprocal Rank Fusion. Keys of queries are
// vector-space names (DefaultVectorSpace or "" targets the default space).
//
// This is the multimodal entry point: e.g. fuse a "text" query and an "image"
// query for the same item. For weighted fusion or to also fold in BM25 text
// results, call SearchSpace / TextSearch and combine the sets with FuseRRF.
func (c *Collection) MultiSpaceSearch(queries map[string][]float32, opts ...SearchOption) ([]Result, error) {
	if len(queries) == 0 {
		return nil, ErrEmptyVector
	}

	config := defaultSearchConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	fetchK := config.effectiveTopK() * 2
	if fetchK < 20 {
		fetchK = 20
	}

	perSpaceOpts := []SearchOption{TopK(fetchK), WithContent(config.includeContent)}
	if config.threshold != nil {
		perSpaceOpts = append(perSpaceOpts, Threshold(*config.threshold))
	}
	for _, f := range config.filters {
		perSpaceOpts = append(perSpaceOpts, WithFilter(f))
	}
	if config.efSearch > 0 {
		perSpaceOpts = append(perSpaceOpts, WithEfSearch(config.efSearch))
	}

	// Deterministic space ordering keeps fused scores reproducible.
	names := make([]string, 0, len(queries))
	for name := range queries {
		names = append(names, name)
	}
	sort.Strings(names)

	sets := make([][]Result, 0, len(queries))
	for _, name := range names {
		if len(queries[name]) == 0 {
			continue
		}
		res, err := c.SearchSpace(name, queries[name], perSpaceOpts...)
		if err != nil {
			return nil, err
		}
		sets = append(sets, res)
	}

	fused := FuseRRF(sets, WithFusionTopK(config.effectiveTopK()))
	return config.applyPagination(fused), nil
}
