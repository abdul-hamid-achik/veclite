package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/veclite"
)

// hoistFlags reorders a command's args so flag tokens precede the leading
// positional arguments. Go's flag package stops parsing at the first
// non-flag argument, so without this, the documented "<file> <collection>
// --flag" ordering would silently drop the flags. Positional arguments are
// always leading in these commands, so moving the leading non-dash tokens to
// the end keeps flag/value adjacency intact for both --flag=v and --flag v.
func hoistFlags(args []string) []string {
	i := 0
	for i < len(args) && !strings.HasPrefix(args[i], "-") {
		i++
	}
	if i == 0 {
		return args
	}
	reordered := make([]string, 0, len(args))
	reordered = append(reordered, args[i:]...)
	reordered = append(reordered, args[:i]...)
	return reordered
}

// parseDistanceType maps a CLI distance name to a veclite.DistanceType.
func parseDistanceType(name string) (veclite.DistanceType, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "cosine":
		return veclite.DistanceCosine, nil
	case "dot":
		return veclite.DistanceDot, nil
	case "euclidean":
		return veclite.DistanceEuclidean, nil
	case "euclidean_squared", "euclidean-squared":
		return veclite.DistanceEuclideanSquared, nil
	default:
		return "", fmt.Errorf("unknown distance type: %s", name)
	}
}

// parseVectorMap parses a JSON object mapping vector-space names to numeric
// arrays, e.g. {"default":[0.1,0.2],"image":[0.3,0.4]}.
func parseVectorMap(s string) (map[string][]float32, error) {
	var raw map[string][]float64
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("parsing vectors: %w", err)
	}
	out := make(map[string][]float32, len(raw))
	for name, vec := range raw {
		conv := make([]float32, len(vec))
		for i, v := range vec {
			conv[i] = float32(v)
		}
		out[name] = conv
	}
	return out, nil
}

// recordInputJSON is the CLI/file representation of a veclite.RecordInput.
type recordInputJSON struct {
	ID      uint64               `json:"id"`
	Content string               `json:"content"`
	Payload map[string]any       `json:"payload"`
	Vectors map[string][]float64 `json:"vectors"`
}

func (r recordInputJSON) toInput() veclite.RecordInput {
	vectors := make(map[string][]float32, len(r.Vectors))
	for name, vec := range r.Vectors {
		conv := make([]float32, len(vec))
		for i, v := range vec {
			conv[i] = float32(v)
		}
		vectors[name] = conv
	}
	return veclite.RecordInput{ID: r.ID, Content: r.Content, Payload: r.Payload, Vectors: vectors}
}

// cmdSpaceAdd declares a named vector space on a collection.
func cmdSpaceAdd(args []string) error {
	fs := flag.NewFlagSet("space-add", flag.ExitOnError)
	name := fs.String("name", "", "Vector space name (required, not 'default')")
	dim := fs.Int("dim", 0, "Vector dimension (0 = auto-detect on first insert)")
	distance := fs.String("distance", "cosine", "Distance metric: cosine, dot, euclidean, euclidean_squared")
	modality := fs.String("modality", "", "Modality hint, e.g. text, image, audio")
	provider := fs.String("provider", "", "Embedding provider")
	model := fs.String("model", "", "Embedding model")
	hnsw := fs.Bool("hnsw", false, "Enable an HNSW index for this space")
	hnswM := fs.Int("hnsw-m", 16, "HNSW M parameter")
	hnswEf := fs.Int("hnsw-ef", 200, "HNSW efConstruction parameter")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite space-add [options] <file> <collection>")
		fmt.Println("\nDeclare an additional named vector space on a collection.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite space-add data.veclite items --name=image --dim=512 --modality=image --hnsw")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}

	dist, err := parseDistanceType(*distance)
	if err != nil {
		return err
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll := db.Collection(collName)

	cfg := veclite.VectorSpaceConfig{
		Name:      *name,
		Dimension: *dim,
		Distance:  dist,
		Modality:  *modality,
		Provider:  *provider,
		Model:     *model,
	}
	if *hnsw {
		cfg.HNSW = &veclite.HNSWConfig{M: *hnswM, EfConstruction: *hnswEf, EfSearch: 100, UseHeuristic: true}
	}

	if err := coll.AddVectorSpace(cfg); err != nil {
		return fmt.Errorf("adding vector space: %w", err)
	}
	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing database: %w", err)
	}

	if *jsonOutput {
		return encodeJSON(map[string]any{
			"status":     "created",
			"collection": collName,
			"space":      *name,
			"dimension":  *dim,
			"distance":   string(dist),
			"modality":   *modality,
			"hnsw":       *hnsw,
		})
	}
	fmt.Printf("Added vector space %q to collection %q\n", *name, collName)
	return nil
}

// cmdSpaces lists the vector spaces of a collection.
func cmdSpaces(args []string) error {
	fs := flag.NewFlagSet("spaces", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite spaces [options] <file> <collection>")
		fmt.Println("\nList the vector spaces of a collection (including the default space).")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll, err := db.GetCollection(collName)
	if err != nil {
		return fmt.Errorf("getting collection: %w", err)
	}

	spaces := coll.VectorSpaces()

	if *jsonOutput {
		type spaceOut struct {
			Name        string `json:"name"`
			Dimension   int    `json:"dimension"`
			Distance    string `json:"distance"`
			Modality    string `json:"modality,omitempty"`
			Provider    string `json:"provider,omitempty"`
			Model       string `json:"model,omitempty"`
			IndexType   string `json:"index_type"`
			VectorCount int    `json:"vector_count"`
		}
		out := make([]spaceOut, len(spaces))
		for i, s := range spaces {
			out[i] = spaceOut{
				Name:        s.Name,
				Dimension:   s.Dimension,
				Distance:    string(s.Distance),
				Modality:    s.Modality,
				Provider:    s.Provider,
				Model:       s.Model,
				IndexType:   s.IndexType,
				VectorCount: s.VectorCount,
			}
		}
		return encodeJSON(out)
	}

	fmt.Printf("Vector spaces in %q:\n", collName)
	for _, s := range spaces {
		fmt.Printf("  %s: dimension=%d, distance=%s, index=%s, vectors=%d",
			s.Name, s.Dimension, s.Distance, s.IndexType, s.VectorCount)
		if s.Modality != "" {
			fmt.Printf(", modality=%s", s.Modality)
		}
		fmt.Println()
	}
	return nil
}

// cmdRecordInsert inserts one logical record carrying vectors in several spaces.
func cmdRecordInsert(args []string) error {
	fs := flag.NewFlagSet("record-insert", flag.ExitOnError)
	vectorsStr := fs.String("vectors", "", `Vectors as JSON object, e.g. '{"default":[0.1,0.2],"image":[0.3,0.4]}'`)
	content := fs.String("content", "", "Text content (indexed by BM25 when text indexing is enabled)")
	payloadStr := fs.String("payload", "", "Payload as JSON object")
	id := fs.Uint64("id", 0, "Record ID (0 = auto-assign; existing ID replaces the record)")
	input := fs.String("input", "", "Read RecordInput JSON (object or array) from a file")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite record-insert [options] <file> <collection>")
		fmt.Println("\nInsert a logical record with vectors in one or more named spaces.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println(`  veclite record-insert data.veclite items --vectors='{"default":[0.1,0.2],"image":[0.3,0.4]}' --content='red apple'`)
		fmt.Println("  veclite record-insert data.veclite items --input=records.json")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	// Build the list of inputs from either --input or the inline flags.
	var inputs []veclite.RecordInput
	if *input != "" {
		data, err := os.ReadFile(*input)
		if err != nil {
			return fmt.Errorf("reading input file: %w", err)
		}
		trimmed := strings.TrimSpace(string(data))
		if strings.HasPrefix(trimmed, "[") {
			var arr []recordInputJSON
			if err := json.Unmarshal(data, &arr); err != nil {
				return fmt.Errorf("parsing input array: %w", err)
			}
			for _, r := range arr {
				inputs = append(inputs, r.toInput())
			}
		} else {
			var one recordInputJSON
			if err := json.Unmarshal(data, &one); err != nil {
				return fmt.Errorf("parsing input object: %w", err)
			}
			inputs = append(inputs, one.toInput())
		}
	} else {
		if *vectorsStr == "" {
			return fmt.Errorf("--vectors is required (or use --input)")
		}
		vectors, err := parseVectorMap(*vectorsStr)
		if err != nil {
			return err
		}
		var payload map[string]any
		if *payloadStr != "" {
			if err := json.Unmarshal([]byte(*payloadStr), &payload); err != nil {
				return fmt.Errorf("parsing payload: %w", err)
			}
		}
		inputs = append(inputs, veclite.RecordInput{ID: *id, Content: *content, Payload: payload, Vectors: vectors})
	}

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll := db.Collection(collName)
	ids := make([]uint64, 0, len(inputs))
	for _, in := range inputs {
		recID, err := coll.InsertRecord(in)
		if err != nil {
			return fmt.Errorf("inserting record: %w", err)
		}
		ids = append(ids, recID)
	}
	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing database: %w", err)
	}

	if *jsonOutput {
		return encodeJSON(map[string]any{
			"status":     "inserted",
			"collection": collName,
			"count":      len(ids),
			"ids":        ids,
		})
	}
	if len(ids) == 1 {
		fmt.Printf("Inserted record with ID: %d\n", ids[0])
	} else {
		fmt.Printf("Inserted %d records\n", len(ids))
	}
	return nil
}

// cmdSearchSpace searches a single named vector space.
func cmdSearchSpace(args []string) error {
	fs := flag.NewFlagSet("search-space", flag.ExitOnError)
	queryStr := fs.String("query", "", "Query vector as JSON array")
	topK := fs.Int("top-k", 10, "Number of results to return")
	threshold := fs.Float64("threshold", 0, "Minimum similarity threshold (0 = disabled)")
	filterStr := fs.String("filter", "", "Filter expression, e.g., 'type=code'")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite search-space [options] <file> <collection> <space>")
		fmt.Println("\nSearch a single named vector space ('default' is the implicit space).")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite search-space data.veclite items image --query='[0.3,0.4]' --top-k=5")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 3 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file, collection and space")
	}
	if *queryStr == "" {
		return fmt.Errorf("--query is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)
	space := fs.Arg(2)

	query, err := parseFloat32Array(*queryStr)
	if err != nil {
		return err
	}

	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll, err := db.GetCollection(collName)
	if err != nil {
		return fmt.Errorf("getting collection: %w", err)
	}

	opts := []veclite.SearchOption{veclite.TopK(*topK)}
	if *threshold > 0 {
		opts = append(opts, veclite.Threshold(float32(*threshold)))
	}
	if *filterStr != "" {
		if f := parseFilter(*filterStr); f != nil {
			opts = append(opts, veclite.WithFilter(f))
		}
	}

	results, err := coll.SearchSpace(space, query, opts...)
	if err != nil {
		return fmt.Errorf("searching space: %w", err)
	}

	return outputSearchResults(results, space, *jsonOutput)
}

// cmdFuseSearch runs one query per space and fuses the result sets with RRF.
func cmdFuseSearch(args []string) error {
	fs := flag.NewFlagSet("fuse-search", flag.ExitOnError)
	queriesStr := fs.String("queries", "", `Queries as JSON object, e.g. '{"default":[0.1,0.2],"image":[0.3,0.4]}'`)
	topK := fs.Int("top-k", 10, "Number of fused results to return")
	filterStr := fs.String("filter", "", "Filter expression applied to all spaces")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite fuse-search [options] <file> <collection>")
		fmt.Println("\nSearch several vector spaces at once and fuse the results with RRF.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println(`  veclite fuse-search data.veclite items --queries='{"default":[0.1,0.2],"image":[0.3,0.4]}'`)
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}
	if *queriesStr == "" {
		return fmt.Errorf("--queries is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	queries, err := parseVectorMap(*queriesStr)
	if err != nil {
		return err
	}

	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll, err := db.GetCollection(collName)
	if err != nil {
		return fmt.Errorf("getting collection: %w", err)
	}

	opts := []veclite.SearchOption{veclite.TopK(*topK)}
	if *filterStr != "" {
		if f := parseFilter(*filterStr); f != nil {
			opts = append(opts, veclite.WithFilter(f))
		}
	}

	results, err := coll.MultiSpaceSearch(queries, opts...)
	if err != nil {
		return fmt.Errorf("fused search: %w", err)
	}

	return outputSearchResults(results, "fused", *jsonOutput)
}

// cmdRecordUpsertByKey inserts or replaces a record identified by a payload key.
func cmdRecordUpsertByKey(args []string) error {
	fs := flag.NewFlagSet("record-upsert-by-key", flag.ExitOnError)
	keyField := fs.String("key-field", "", "Payload key whose value identifies the record (required)")
	keyValue := fs.String("key-value", "", "Value of the payload key (required)")
	vectorsStr := fs.String("vectors", "", `Vectors as JSON object, e.g. '{"default":[0.1,0.2],"image":[0.3,0.4]}'`)
	content := fs.String("content", "", "Text content (indexed by BM25 when text indexing is enabled)")
	payloadStr := fs.String("payload", "", "Payload as JSON object")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite record-upsert-by-key [options] <file> <collection>")
		fmt.Println("\nInsert or replace a logical record identified by a payload key,")
		fmt.Println("carrying vectors in one or more named spaces. Idempotent by keyField/keyValue.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println(`  veclite record-upsert-by-key data.veclite evidence --key-field=evidence_id --key-value=doc-1 --vectors='{"text":[0.1,0.2,0.3]}' --content='checkout fails'`)
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}
	if *keyField == "" {
		return fmt.Errorf("--key-field is required")
	}
	if *keyValue == "" {
		return fmt.Errorf("--key-value is required")
	}
	if *vectorsStr == "" {
		return fmt.Errorf("--vectors is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	vectors, err := parseVectorMap(*vectorsStr)
	if err != nil {
		return err
	}
	var payload map[string]any
	if *payloadStr != "" {
		if err := json.Unmarshal([]byte(*payloadStr), &payload); err != nil {
			return fmt.Errorf("parsing payload: %w", err)
		}
	}
	in := veclite.RecordInput{Content: *content, Payload: payload, Vectors: vectors}

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll := db.Collection(collName)
	id, inserted, err := coll.UpsertRecordByKey(*keyField, *keyValue, in)
	if err != nil {
		return fmt.Errorf("upserting record by key: %w", err)
	}
	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing database: %w", err)
	}

	if *jsonOutput {
		return encodeJSON(map[string]any{
			"status":     statusString(inserted),
			"collection": collName,
			"id":         id,
			"inserted":   inserted,
			"key_field":  *keyField,
			"key_value":  *keyValue,
		})
	}
	fmt.Printf("Upserted record by %s=%s: id=%d, inserted=%v\n", *keyField, *keyValue, id, inserted)
	return nil
}

// cmdHybridSearchSpace runs a vector search over a named space and a BM25 text
// search, then fuses the results with RRF.
func cmdHybridSearchSpace(args []string) error {
	fs := flag.NewFlagSet("hybrid-search-space", flag.ExitOnError)
	queryStr := fs.String("query", "", "Query vector as JSON array (required)")
	textQuery := fs.String("text", "", "Text query for BM25 (required)")
	topK := fs.Int("top-k", 10, "Number of fused results to return")
	threshold := fs.Float64("threshold", 0, "Minimum similarity threshold (0 = disabled)")
	filterStr := fs.String("filter", "", "Filter expression applied to both halves")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite hybrid-search-space [options] <file> <collection> <space>")
		fmt.Println("\nSearch a named vector space and BM25, then fuse with RRF.")
		fmt.Println("'default' (or omitted space) is equivalent to hybrid-search.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite hybrid-search-space data.veclite evidence text --query='[0.1,0.2,0.3]' --text='checkout' --top-k=5")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection (space is optional, defaults to 'default')")
	}
	if *queryStr == "" {
		return fmt.Errorf("--query is required")
	}
	if *textQuery == "" {
		return fmt.Errorf("--text is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)
	space := veclite.DefaultVectorSpace
	if fs.NArg() >= 3 {
		space = fs.Arg(2)
	}

	query, err := parseFloat32Array(*queryStr)
	if err != nil {
		return err
	}

	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll, err := db.GetCollection(collName)
	if err != nil {
		return fmt.Errorf("getting collection: %w", err)
	}

	opts := []veclite.SearchOption{veclite.TopK(*topK)}
	if *threshold > 0 {
		opts = append(opts, veclite.Threshold(float32(*threshold)))
	}
	if *filterStr != "" {
		if f := parseFilter(*filterStr); f != nil {
			opts = append(opts, veclite.WithFilter(f))
		}
	}

	results, err := coll.HybridSearchSpace(space, query, *textQuery, opts...)
	if err != nil {
		return fmt.Errorf("hybrid space search: %w", err)
	}

	return outputSearchResults(results, "hybrid:"+space, *jsonOutput)
}

func statusString(inserted bool) string {
	if inserted {
		return "inserted"
	}
	return "replaced"
}

// parseFloat32Array parses a JSON numeric array into []float32.
func parseFloat32Array(s string) ([]float32, error) {
	var f64 []float64
	if err := json.Unmarshal([]byte(s), &f64); err != nil {
		return nil, fmt.Errorf("parsing vector: %w", err)
	}
	out := make([]float32, len(f64))
	for i, v := range f64 {
		out[i] = float32(v)
	}
	return out, nil
}

// outputSearchResults renders search results as text or JSON.
func outputSearchResults(results []veclite.Result, label string, asJSON bool) error {
	if asJSON {
		type resultOutput struct {
			ID      uint64         `json:"id"`
			Score   float32        `json:"score"`
			Payload map[string]any `json:"payload,omitempty"`
		}
		out := make([]resultOutput, len(results))
		for i, r := range results {
			out[i] = resultOutput{ID: r.Record.ID, Score: r.Score, Payload: r.Record.Payload}
		}
		return encodeJSON(out)
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}
	fmt.Printf("Found %d results in %s:\n", len(results), label)
	for i, r := range results {
		fmt.Printf("%d. ID=%d, score=%.4f", i+1, r.Record.ID, r.Score)
		if r.Record.Payload != nil {
			payloadJSON, _ := json.Marshal(r.Record.Payload)
			fmt.Printf(", payload=%s", payloadJSON)
		}
		fmt.Println()
	}
	return nil
}

// encodeJSON writes v to stdout as indented JSON.
func encodeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
