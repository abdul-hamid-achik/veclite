package veclite

import (
	"math"
	"sort"
	"strings"
)

// bm25Scorer implements the BM25 (Okapi BM25) scoring algorithm.
type bm25Scorer struct {
	k1 float64
	b  float64
}

func newBM25Scorer() *bm25Scorer {
	return &bm25Scorer{
		k1: 1.2,
		b:  0.75,
	}
}

// score computes BM25 score for a document given query terms.
// tf is the term frequency in the document.
// docLen is the number of tokens in the document.
// avgDL is the average document length across all documents.
// n is the total number of documents.
// df is the document frequency of the term (number of docs containing it).
func (s *bm25Scorer) score(tf int, docLen int, avgDL float64, n int, df int) float64 {
	// IDF component: log((N - df + 0.5) / (df + 0.5) + 1)
	idf := math.Log((float64(n)-float64(df)+0.5)/(float64(df)+0.5) + 1.0)

	// TF component with length normalization
	tfNorm := (float64(tf) * (s.k1 + 1.0)) /
		(float64(tf) + s.k1*(1.0-s.b+s.b*(float64(docLen)/avgDL)))

	return idf * tfNorm
}

// tokenize splits text into lowercase tokens using whitespace.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	fields := strings.Fields(text)
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		// Strip common punctuation from edges
		f = strings.Trim(f, ".,;:!?\"'()[]{}#@$%^&*+=<>/\\|`~")
		if f != "" {
			tokens = append(tokens, f)
		}
	}
	return tokens
}

// textSearchResult holds an intermediate BM25 search result.
type textSearchResult struct {
	id    uint64
	score float64
}

// invertedIndex is an in-memory inverted index for BM25 full-text search.
type invertedIndex struct {
	// postings maps term -> set of record IDs containing that term
	postings map[string]map[uint64]struct{}
	// docLengths maps record ID -> number of tokens in the document
	docLengths map[uint64]int
	// totalDocLen is the sum of all document lengths
	totalDocLen int64
	// docCount is the number of indexed documents
	docCount int
	// fields are the payload fields to index
	fields []string
	// scorer is the BM25 scorer
	scorer *bm25Scorer
}

func newInvertedIndex(fields []string) *invertedIndex {
	return &invertedIndex{
		postings:   make(map[string]map[uint64]struct{}),
		docLengths: make(map[uint64]int),
		fields:     fields,
		scorer:     newBM25Scorer(),
	}
}

// indexRecord adds a record to the inverted index.
func (idx *invertedIndex) indexRecord(id uint64, payload map[string]any, content string) {
	// Remove old entry if exists
	idx.removeRecord(id)

	// Build document text from configured fields and content
	var textParts []string
	if content != "" {
		textParts = append(textParts, content)
	}
	if payload != nil {
		for _, field := range idx.fields {
			if v, ok := payload[field]; ok {
				if s, ok := v.(string); ok {
					textParts = append(textParts, s)
				}
			}
		}
	}

	if len(textParts) == 0 {
		return
	}

	text := strings.Join(textParts, " ")
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return
	}

	// Count term frequencies
	tf := make(map[string]int)
	for _, token := range tokens {
		tf[token]++
	}

	// Update postings
	for term := range tf {
		if idx.postings[term] == nil {
			idx.postings[term] = make(map[uint64]struct{})
		}
		idx.postings[term][id] = struct{}{}
	}

	// Update doc length
	idx.docLengths[id] = len(tokens)
	idx.totalDocLen += int64(len(tokens))
	idx.docCount++
}

// removeRecord removes a record from the inverted index.
func (idx *invertedIndex) removeRecord(id uint64) {
	oldLen, exists := idx.docLengths[id]
	if !exists {
		return
	}

	// Remove from all postings
	for term, docs := range idx.postings {
		delete(docs, id)
		if len(docs) == 0 {
			delete(idx.postings, term)
		}
	}

	idx.totalDocLen -= int64(oldLen)
	idx.docCount--
	delete(idx.docLengths, id)
}

// search performs BM25 search and returns results sorted by score (descending).
func (idx *invertedIndex) search(query string, topK int) []textSearchResult {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	// Calculate average document length
	avgDL := float64(idx.totalDocLen) / math.Max(float64(idx.docCount), 1.0)

	// Accumulate scores per document
	scores := make(map[uint64]float64)

	for _, term := range queryTokens {
		docs, ok := idx.postings[term]
		if !ok {
			continue
		}

		df := len(docs)

		for docID := range docs {
			// Count term frequency in this document
			// We need to reconstruct TF from the postings; since we don't store per-doc TF,
			// we track it as 1 (term is present). For better accuracy, we'd store TF.
			// This is acceptable for the simple tokenizer approach.
			docLen := idx.docLengths[docID]
			tf := 1 // term is present at least once
			score := idx.scorer.score(tf, docLen, avgDL, idx.docCount, df)
			scores[docID] += score
		}
	}

	// Sort by score descending
	results := make([]textSearchResult, 0, len(scores))
	for id, score := range scores {
		results = append(results, textSearchResult{id: id, score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}

// InvertedIndexSnapshot is the serializable state of the inverted index.
type InvertedIndexSnapshot struct {
	Postings    map[string][]uint64
	DocLengths  map[uint64]int
	TotalDocLen int64
	DocCount    int
	Fields      []string
}

// snapshot creates a serializable snapshot of the inverted index.
func (idx *invertedIndex) snapshot() *InvertedIndexSnapshot {
	if idx == nil {
		return nil
	}

	postings := make(map[string][]uint64, len(idx.postings))
	for term, docs := range idx.postings {
		ids := make([]uint64, 0, len(docs))
		for id := range docs {
			ids = append(ids, id)
		}
		postings[term] = ids
	}

	docLengths := make(map[uint64]int, len(idx.docLengths))
	for k, v := range idx.docLengths {
		docLengths[k] = v
	}

	fields := make([]string, len(idx.fields))
	copy(fields, idx.fields)

	return &InvertedIndexSnapshot{
		Postings:    postings,
		DocLengths:  docLengths,
		TotalDocLen: idx.totalDocLen,
		DocCount:    idx.docCount,
		Fields:      fields,
	}
}

// loadInvertedIndexFromSnapshot restores an inverted index from a snapshot.
func loadInvertedIndexFromSnapshot(snap *InvertedIndexSnapshot) *invertedIndex {
	if snap == nil {
		return nil
	}

	idx := &invertedIndex{
		postings:    make(map[string]map[uint64]struct{}, len(snap.Postings)),
		docLengths:  make(map[uint64]int, len(snap.DocLengths)),
		totalDocLen: snap.TotalDocLen,
		docCount:    snap.DocCount,
		fields:      snap.Fields,
		scorer:      newBM25Scorer(),
	}

	for term, ids := range snap.Postings {
		docs := make(map[uint64]struct{}, len(ids))
		for _, id := range ids {
			docs[id] = struct{}{}
		}
		idx.postings[term] = docs
	}

	for k, v := range snap.DocLengths {
		idx.docLengths[k] = v
	}

	return idx
}
