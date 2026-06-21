// Command veclite provides a CLI for interacting with VecLite databases.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/abdul-hamid-achik/veclite"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	cmd := os.Args[1]
	// Reorder so flags placed after the leading positional arguments are still
	// parsed. Go's flag package stops at the first non-flag token, so without this
	// the documented "<command> <file> <collection> --flag" ordering would silently
	// drop the flags. hoistFlags (cmd/veclite/spaces.go) is idempotent and a no-op
	// when args already lead with flags, so flags-first invocations are unaffected.
	args := hoistFlags(os.Args[2:])
	switch cmd {
	case "version":
		cmdVersion()
		return
	case "info":
		err = cmdInfo(args)
	case "collections":
		err = cmdCollections(args)
	case "stats":
		err = cmdStats(args)
	case "dump":
		err = cmdDump(args)
	case "create-collection":
		err = cmdCreateCollection(args)
	case "drop-collection":
		err = cmdDropCollection(args)
	case "insert":
		err = cmdInsert(args)
	case "batch-insert":
		err = cmdBatchInsert(args)
	case "search":
		err = cmdSearch(args)
	case "delete":
		err = cmdDelete(args)
	case "get":
		err = cmdGet(args)
	case "upsert":
		err = cmdUpsert(args)
	case "update":
		err = cmdUpdate(args)
	case "delete-where":
		err = cmdDeleteWhere(args)
	case "find":
		err = cmdFind(args)
	case "space-add":
		err = cmdSpaceAdd(args)
	case "spaces":
		err = cmdSpaces(args)
	case "record-insert":
		err = cmdRecordInsert(args)
	case "search-space":
		err = cmdSearchSpace(args)
	case "fuse-search":
		err = cmdFuseSearch(args)
	case "record-upsert-by-key":
		err = cmdRecordUpsertByKey(args)
	case "hybrid-search-space":
		err = cmdHybridSearchSpace(args)
	case "serve":
		err = cmdServe(args)
	case "mcp":
		err = cmdMCP(args)
	case "compact":
		err = cmdCompact(args)
	case "validate":
		err = cmdValidate(args)
	case "benchmark":
		err = cmdBenchmark(args)
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`veclite - VecLite database CLI

Usage:
  veclite <command> [arguments]

Read Commands:
  version                  Show version information
  info <file>              Show database information
  collections <file>       List all collections
  stats <file>             Show detailed statistics
  dump <file>              Dump database contents as JSON
  get <file> <collection>  Get a vector by ID

Write Commands:
  create-collection <file> <name>  Create a new collection
  drop-collection <file> <name>    Drop a collection
  insert <file> <collection>       Insert a vector
  batch-insert <file> <collection> Insert vectors from JSON file
  upsert <file> <collection>       Upsert a vector (insert or update)
  update <file> <collection>       Update vector and/or payload
  delete <file> <collection>       Delete a vector by ID
  delete-where <file> <collection> Delete vectors matching a filter
  find <file> <collection>         Find records by filter (no vector needed)
  search <file> <collection>       Search for similar vectors

Named Vector Spaces:
  spaces <file> <collection>       List a collection's vector spaces
  space-add <file> <collection>    Declare a named vector space (--name, --dim, ...)
  record-insert <file> <collection> Insert a record with vectors in several spaces
  record-upsert-by-key <file> <collection>  Insert or replace a record by payload key
  search-space <file> <collection> <space>  Search one named vector space
  fuse-search <file> <collection>  Search several spaces and fuse with RRF
  hybrid-search-space <file> <collection> [space]  Search a named space + BM25, fuse with RRF

Server Mode:
  serve <file>             Start HTTP server for multi-client access
  mcp <file>               Start MCP tool server over stdio

Maintenance Commands:
  compact <file>           Compact database and reclaim space
  validate <file>          Validate database integrity
  benchmark <file>         Run search performance benchmark

Other:
  help                     Show this help message

Global Flags:
  --json                   Output results as JSON (supported by most commands)

Examples:
  veclite version
  veclite info data.veclite
  veclite create-collection data.veclite embeddings --dimension=384 --distance=cosine
  veclite insert data.veclite embeddings --vector='[0.1,0.2,0.3]' --payload='{"file":"main.go"}'
  veclite search data.veclite embeddings --query='[0.1,0.2,0.3]' --top-k=5
  veclite space-add data.veclite items --name=image --dim=512 --modality=image --hnsw
  veclite record-insert data.veclite items --vectors='{"default":[0.1,0.2],"image":[0.3,0.4]}'
  veclite search-space data.veclite items image --query='[0.3,0.4]' --top-k=5
  veclite fuse-search data.veclite items --queries='{"default":[0.1,0.2],"image":[0.3,0.4]}'
  veclite record-upsert-by-key data.veclite evidence --key-field=evidence_id --key-value=doc-1 --vectors='{"text":[0.1,0.2,0.3]}' --content='checkout fails'
  veclite hybrid-search-space data.veclite evidence text --query='[0.1,0.2,0.3]' --text='checkout' --top-k=5
  veclite serve data.veclite --port=8080`)
}

func cmdVersion() {
	fmt.Printf("veclite version %s (library %s)\n", version, veclite.Version)
	if commit != "none" {
		fmt.Printf("commit: %s\n", commit)
	}
}

func cmdInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite info [options] <file>")
		fmt.Println("\nShow database information.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}

	path := fs.Arg(0)
	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	stats := db.Stats()

	if *jsonOutput {
		info := map[string]any{
			"path":          path,
			"collections":   stats.Collections,
			"total_records": stats.TotalRecords,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(info)
		return nil
	}

	fmt.Printf("Database: %s\n", path)
	fmt.Printf("Collections: %d\n", stats.Collections)
	fmt.Printf("Total Records: %d\n", stats.TotalRecords)
	return nil
}

func cmdCollections(args []string) error {
	fs := flag.NewFlagSet("collections", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite collections [options] <file>")
		fmt.Println("\nList all collections in the database.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}

	path := fs.Arg(0)
	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	collections := db.Collections()

	if *jsonOutput {
		type collInfo struct {
			Name      string `json:"name"`
			Count     int    `json:"count"`
			Dimension int    `json:"dimension"`
			Distance  string `json:"distance"`
			IndexType string `json:"index_type"`
		}
		result := make([]collInfo, 0, len(collections))
		for _, name := range collections {
			coll, _ := db.GetCollection(name)
			stats := coll.Stats()
			result = append(result, collInfo{
				Name:      name,
				Count:     stats.Count,
				Dimension: stats.Dimension,
				Distance:  stats.DistanceType,
				IndexType: stats.IndexType,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	if len(collections) == 0 {
		fmt.Println("No collections")
		return nil
	}

	for _, name := range collections {
		coll, _ := db.GetCollection(name)
		stats := coll.Stats()
		fmt.Printf("%s: %d records, dimension=%d, distance=%s, index=%s\n",
			name, stats.Count, stats.Dimension, stats.DistanceType, stats.IndexType)
	}
	return nil
}

func cmdStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite stats [options] <file>")
		fmt.Println("\nShow detailed statistics.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}

	path := fs.Arg(0)
	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	stats := db.Stats()

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(stats)
		return nil
	}

	fmt.Printf("Database: %s\n", stats.Path)
	fmt.Printf("Collections: %d\n", stats.Collections)
	fmt.Printf("Total Records: %d\n", stats.TotalRecords)
	fmt.Println()

	if len(stats.CollectionStats) > 0 {
		fmt.Println("Collection Details:")
		for _, cs := range stats.CollectionStats {
			fmt.Printf("  %s:\n", cs.Name)
			fmt.Printf("    Records: %d\n", cs.Count)
			fmt.Printf("    Dimension: %d\n", cs.Dimension)
			fmt.Printf("    Distance: %s\n", cs.DistanceType)
			fmt.Printf("    Index: %s\n", cs.IndexType)
		}
	}
	return nil
}

func cmdDump(args []string) error {
	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	collection := fs.String("collection", "", "Dump specific collection")
	limit := fs.Int("limit", 0, "Limit number of records (0 = all)")
	fs.Usage = func() {
		fmt.Println("Usage: veclite dump [options] <file>")
		fmt.Println("\nDump database contents as JSON.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}

	path := fs.Arg(0)
	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	type recordDump struct {
		ID      uint64         `json:"id"`
		Vector  []float32      `json:"vector"`
		Payload map[string]any `json:"payload,omitempty"`
	}

	type collectionDump struct {
		Name      string       `json:"name"`
		Dimension int          `json:"dimension"`
		Distance  string       `json:"distance"`
		IndexType string       `json:"index_type"`
		Count     int          `json:"count"`
		Records   []recordDump `json:"records"`
	}

	type databaseDump struct {
		Path        string           `json:"path"`
		Collections []collectionDump `json:"collections"`
	}

	dump := databaseDump{
		Path:        path,
		Collections: make([]collectionDump, 0),
	}

	collections := db.Collections()
	for _, name := range collections {
		if *collection != "" && name != *collection {
			continue
		}

		coll, _ := db.GetCollection(name)
		stats := coll.Stats()
		records := coll.All()

		collDump := collectionDump{
			Name:      name,
			Dimension: stats.Dimension,
			Distance:  stats.DistanceType,
			IndexType: stats.IndexType,
			Count:     stats.Count,
			Records:   make([]recordDump, 0),
		}

		for i, r := range records {
			if *limit > 0 && i >= *limit {
				break
			}
			collDump.Records = append(collDump.Records, recordDump{
				ID:      r.ID,
				Vector:  r.Vector,
				Payload: r.Payload,
			})
		}

		dump.Collections = append(dump.Collections, collDump)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(dump)
	return nil
}

func cmdCreateCollection(args []string) error {
	fs := flag.NewFlagSet("create-collection", flag.ExitOnError)
	dimension := fs.Int("dimension", 0, "Vector dimension (0 = auto-detect on first insert)")
	distance := fs.String("distance", "cosine", "Distance metric: cosine, dot, euclidean")
	hnsw := fs.Bool("hnsw", false, "Enable HNSW indexing")
	hnswM := fs.Int("hnsw-m", 16, "HNSW M parameter (max connections per node)")
	hnswEf := fs.Int("hnsw-ef", 200, "HNSW efConstruction parameter")
	textIndex := fs.String("text-index", "", "Comma-separated payload fields to BM25-index in addition to Content (enables TextSearch/HybridSearch)")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite create-collection [options] <file> <name>")
		fmt.Println("\nCreate a new collection.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and name")
	}

	path := fs.Arg(0)
	name := fs.Arg(1)

	// Parse distance type
	var distType veclite.DistanceType
	switch strings.ToLower(*distance) {
	case "cosine":
		distType = veclite.DistanceCosine
	case "dot":
		distType = veclite.DistanceDot
	case "euclidean":
		distType = veclite.DistanceEuclidean
	default:
		return fmt.Errorf("unknown distance type: %s", *distance)
	}

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Build collection options
	opts := []veclite.CollectionOption{
		veclite.WithDistanceType(distType),
	}
	if *dimension > 0 {
		opts = append(opts, veclite.WithDimension(*dimension))
	}
	if *hnsw {
		opts = append(opts, veclite.WithHNSW(*hnswM, *hnswEf))
	}
	if *textIndex != "" {
		fields := strings.Split(*textIndex, ",")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		opts = append(opts, veclite.WithTextIndex(fields...))
	}

	_, err = db.CreateCollection(name, opts...)
	if err != nil {
		return fmt.Errorf("creating collection: %w", err)
	}

	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing database: %w", err)
	}

	if *jsonOutput {
		result := map[string]any{
			"status":     "created",
			"collection": name,
			"dimension":  *dimension,
			"distance":   *distance,
			"hnsw":       *hnsw,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	fmt.Printf("Created collection: %s\n", name)
	return nil
}

func cmdDropCollection(args []string) error {
	fs := flag.NewFlagSet("drop-collection", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite drop-collection [options] <file> <name>")
		fmt.Println("\nDrop a collection and all its data.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and name")
	}

	path := fs.Arg(0)
	name := fs.Arg(1)

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	err = db.DropCollection(name)
	if err != nil {
		return fmt.Errorf("dropping collection: %w", err)
	}

	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing database: %w", err)
	}

	if *jsonOutput {
		result := map[string]any{
			"status":     "dropped",
			"collection": name,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	fmt.Printf("Dropped collection: %s\n", name)
	return nil
}

func cmdInsert(args []string) error {
	fs := flag.NewFlagSet("insert", flag.ExitOnError)
	vectorStr := fs.String("vector", "", "Vector as JSON array, e.g., '[0.1,0.2,0.3]'")
	payloadStr := fs.String("payload", "", "Payload as JSON object, e.g., '{\"file\":\"main.go\"}'")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite insert [options] <file> <collection>")
		fmt.Println("\nInsert a vector into a collection.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite insert data.veclite embeddings --vector='[0.1,0.2,0.3]'")
		fmt.Println("  veclite insert data.veclite embeddings --vector='[0.1,0.2,0.3]' --payload='{\"file\":\"main.go\"}'")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}

	if *vectorStr == "" {
		return fmt.Errorf("--vector is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	// Parse vector
	var vector []float32
	if err := json.Unmarshal([]byte(*vectorStr), &vector); err != nil {
		// Try parsing as float64 array
		var vector64 []float64
		if err2 := json.Unmarshal([]byte(*vectorStr), &vector64); err2 != nil {
			return fmt.Errorf("parsing vector: %w", err)
		}
		vector = make([]float32, len(vector64))
		for i, v := range vector64 {
			vector[i] = float32(v)
		}
	}

	// Parse payload
	var payload map[string]any
	if *payloadStr != "" {
		if err := json.Unmarshal([]byte(*payloadStr), &payload); err != nil {
			return fmt.Errorf("parsing payload: %w", err)
		}
	}

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll := db.Collection(collName)
	id, err := coll.Insert(vector, payload)
	if err != nil {
		return fmt.Errorf("inserting vector: %w", err)
	}

	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing database: %w", err)
	}

	if *jsonOutput {
		result := map[string]any{
			"status":     "inserted",
			"id":         id,
			"collection": collName,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	fmt.Printf("Inserted vector with ID: %d\n", id)
	return nil
}

func cmdBatchInsert(args []string) error {
	fs := flag.NewFlagSet("batch-insert", flag.ExitOnError)
	inputFile := fs.String("input", "", "Input file (JSON array or JSONL format)")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite batch-insert [options] <file> <collection>")
		fmt.Println("\nInsert multiple vectors from a JSON file.")
		fmt.Println("\nInput formats supported:")
		fmt.Println("  JSON array: [{\"vector\": [...], \"payload\": {...}}, ...]")
		fmt.Println("  JSONL: one JSON object per line")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite batch-insert data.veclite embeddings --input=vectors.json")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}

	if *inputFile == "" {
		return fmt.Errorf("--input is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	// Read input file
	file, err := os.Open(*inputFile)
	if err != nil {
		return fmt.Errorf("opening input file: %w", err)
	}
	defer file.Close()

	type vectorInput struct {
		Vector  []float64      `json:"vector"`
		Payload map[string]any `json:"payload"`
	}

	var inputs []vectorInput

	// Try to detect format
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("reading input file: %w", err)
	}

	// Try JSON array first
	if err := json.Unmarshal(content, &inputs); err != nil {
		// Try JSONL
		inputs = nil
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var input vectorInput
			if err := json.Unmarshal([]byte(line), &input); err != nil {
				return fmt.Errorf("parsing line: %w", err)
			}
			inputs = append(inputs, input)
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
	}

	if len(inputs) == 0 {
		return fmt.Errorf("no vectors found in input file")
	}

	// Convert to float32 and prepare for batch insert
	vectors := make([][]float32, len(inputs))
	payloads := make([]map[string]any, len(inputs))
	for i, input := range inputs {
		vectors[i] = make([]float32, len(input.Vector))
		for j, v := range input.Vector {
			vectors[i][j] = float32(v)
		}
		payloads[i] = input.Payload
	}

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll := db.Collection(collName)
	ids, err := coll.InsertBatch(vectors, payloads)
	if err != nil {
		return fmt.Errorf("inserting vectors: %w", err)
	}

	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing database: %w", err)
	}

	if *jsonOutput {
		result := map[string]any{
			"status":     "inserted",
			"count":      len(ids),
			"ids":        ids,
			"collection": collName,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	fmt.Printf("Inserted %d vectors (IDs: %d-%d)\n", len(ids), ids[0], ids[len(ids)-1])
	return nil
}

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	queryStr := fs.String("query", "", "Query vector as JSON array")
	topK := fs.Int("top-k", 10, "Number of results to return")
	threshold := fs.Float64("threshold", 0, "Minimum similarity threshold (0 = disabled)")
	filterStr := fs.String("filter", "", "Filter expression, e.g., 'type=code' or 'file=*.go'")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite search [options] <file> <collection>")
		fmt.Println("\nSearch for similar vectors.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nFilter syntax:")
		fmt.Println("  key=value       Exact match")
		fmt.Println("  key=*.ext       Glob pattern match")
		fmt.Println("\nExamples:")
		fmt.Println("  veclite search data.veclite embeddings --query='[0.1,0.2,0.3]' --top-k=5")
		fmt.Println("  veclite search data.veclite embeddings --query='[0.1,0.2,0.3]' --filter='type=code'")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}

	if *queryStr == "" {
		return fmt.Errorf("--query is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	// Parse query vector
	var query []float32
	if err := json.Unmarshal([]byte(*queryStr), &query); err != nil {
		var query64 []float64
		if err2 := json.Unmarshal([]byte(*queryStr), &query64); err2 != nil {
			return fmt.Errorf("parsing query: %w", err)
		}
		query = make([]float32, len(query64))
		for i, v := range query64 {
			query[i] = float32(v)
		}
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

	// Build search options
	searchOpts := []veclite.SearchOption{
		veclite.TopK(*topK),
	}
	if *threshold > 0 {
		searchOpts = append(searchOpts, veclite.Threshold(float32(*threshold)))
	}

	// Parse filter
	if *filterStr != "" {
		filter := parseFilter(*filterStr)
		if filter != nil {
			searchOpts = append(searchOpts, veclite.WithFilter(filter))
		}
	}

	results, err := coll.Search(query, searchOpts...)
	if err != nil {
		return fmt.Errorf("searching: %w", err)
	}

	if *jsonOutput {
		type resultOutput struct {
			ID      uint64         `json:"id"`
			Score   float32        `json:"score"`
			Payload map[string]any `json:"payload,omitempty"`
		}
		output := make([]resultOutput, len(results))
		for i, r := range results {
			output[i] = resultOutput{
				ID:      r.Record.ID,
				Score:   r.Score,
				Payload: r.Record.Payload,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(output)
		return nil
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	fmt.Printf("Found %d results:\n", len(results))
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

func cmdDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	id := fs.Uint64("id", 0, "ID of vector to delete")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite delete [options] <file> <collection>")
		fmt.Println("\nDelete a vector by ID.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite delete data.veclite embeddings --id=42")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}

	if *id == 0 {
		return fmt.Errorf("--id is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll, err := db.GetCollection(collName)
	if err != nil {
		return fmt.Errorf("getting collection: %w", err)
	}

	err = coll.Delete(*id)
	if err != nil {
		return fmt.Errorf("deleting vector: %w", err)
	}

	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing database: %w", err)
	}

	if *jsonOutput {
		result := map[string]any{
			"status":     "deleted",
			"id":         *id,
			"collection": collName,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	fmt.Printf("Deleted vector with ID: %d\n", *id)
	return nil
}

func cmdGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	id := fs.Uint64("id", 0, "ID of vector to get")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite get [options] <file> <collection>")
		fmt.Println("\nGet a vector by ID.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite get data.veclite embeddings --id=42")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}

	if *id == 0 {
		return fmt.Errorf("--id is required")
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

	record, err := coll.Get(*id)
	if err != nil {
		return fmt.Errorf("getting vector: %w", err)
	}

	if *jsonOutput {
		result := map[string]any{
			"id":         record.ID,
			"vector":     record.Vector,
			"payload":    record.Payload,
			"created_at": record.CreatedAt,
			"updated_at": record.UpdatedAt,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	fmt.Printf("ID: %d\n", record.ID)
	fmt.Printf("Vector: %v\n", record.Vector)
	if record.Payload != nil {
		payloadJSON, _ := json.Marshal(record.Payload)
		fmt.Printf("Payload: %s\n", payloadJSON)
	}
	fmt.Printf("Created: %s\n", record.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", record.UpdatedAt.Format("2006-01-02 15:04:05"))
	return nil
}

// parseFilter parses a simple filter expression like "key=value" or "key=*.ext"
func parseFilter(expr string) veclite.Filter {
	parts := strings.SplitN(expr, "=", 2)
	if len(parts) != 2 {
		return nil
	}
	key := parts[0]
	value := parts[1]

	// Check if it's a glob pattern
	if strings.Contains(value, "*") || strings.Contains(value, "?") {
		return veclite.Glob(key, value)
	}

	// Try to parse as number
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return veclite.Equal(key, i)
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return veclite.Equal(key, f)
	}

	// Treat as string
	return veclite.Equal(key, value)
}

func cmdUpsert(args []string) error {
	fs := flag.NewFlagSet("upsert", flag.ExitOnError)
	id := fs.Uint64("id", 0, "Record ID (0 = auto-assign)")
	vectorStr := fs.String("vector", "", "Vector as JSON array, e.g., '[0.1,0.2,0.3]'")
	payloadStr := fs.String("payload", "", "Payload as JSON object")
	keyField := fs.String("key-field", "", "Upsert by key field (uses UpsertByKey)")
	keyValue := fs.String("key-value", "", "Key field value for UpsertByKey")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite upsert [options] <file> <collection>")
		fmt.Println("\nInsert or update a vector.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite upsert data.veclite embeddings --id=5 --vector='[0.1,0.2,0.3]'")
		fmt.Println("  veclite upsert data.veclite embeddings --key-field=file --key-value=main.go --vector='[0.1,0.2,0.3]'")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}
	if *vectorStr == "" {
		return fmt.Errorf("--vector is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	var vector []float32
	if err := json.Unmarshal([]byte(*vectorStr), &vector); err != nil {
		var vector64 []float64
		if err2 := json.Unmarshal([]byte(*vectorStr), &vector64); err2 != nil {
			return fmt.Errorf("parsing vector: %w", err)
		}
		vector = make([]float32, len(vector64))
		for i, v := range vector64 {
			vector[i] = float32(v)
		}
	}

	var payload map[string]any
	if *payloadStr != "" {
		if err := json.Unmarshal([]byte(*payloadStr), &payload); err != nil {
			return fmt.Errorf("parsing payload: %w", err)
		}
	}

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll := db.Collection(collName)

	if *keyField != "" {
		resultID, inserted, err := coll.UpsertByKey(*keyField, *keyValue, vector, payload)
		if err != nil {
			return fmt.Errorf("upserting: %w", err)
		}
		if err := db.Sync(); err != nil {
			return fmt.Errorf("syncing: %w", err)
		}
		action := "updated"
		if inserted {
			action = "inserted"
		}
		if *jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"status": action, "id": resultID, "collection": collName})
			return nil
		}
		fmt.Printf("Upserted (%s) vector with ID: %d\n", action, resultID)
		return nil
	}

	resultID, err := coll.Upsert(*id, vector, payload)
	if err != nil {
		return fmt.Errorf("upserting: %w", err)
	}
	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"status": "upserted", "id": resultID, "collection": collName})
		return nil
	}
	fmt.Printf("Upserted vector with ID: %d\n", resultID)
	return nil
}

func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	id := fs.Uint64("id", 0, "ID of vector to update")
	vectorStr := fs.String("vector", "", "New vector as JSON array")
	payloadStr := fs.String("payload", "", "New payload as JSON object")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite update [options] <file> <collection>")
		fmt.Println("\nUpdate a vector's data and/or payload.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite update data.veclite embeddings --id=5 --payload='{\"file\":\"main.go\"}'")
		fmt.Println("  veclite update data.veclite embeddings --id=5 --vector='[0.1,0.2,0.3]'")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}
	if *id == 0 {
		return fmt.Errorf("--id is required")
	}
	if *vectorStr == "" && *payloadStr == "" {
		return fmt.Errorf("at least one of --vector or --payload is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll, err := db.GetCollection(collName)
	if err != nil {
		return fmt.Errorf("getting collection: %w", err)
	}

	if *vectorStr != "" {
		var vector []float32
		if err := json.Unmarshal([]byte(*vectorStr), &vector); err != nil {
			var vector64 []float64
			if err2 := json.Unmarshal([]byte(*vectorStr), &vector64); err2 != nil {
				return fmt.Errorf("parsing vector: %w", err)
			}
			vector = make([]float32, len(vector64))
			for i, v := range vector64 {
				vector[i] = float32(v)
			}
		}
		if err := coll.UpdateVector(*id, vector); err != nil {
			return fmt.Errorf("updating vector: %w", err)
		}
	}

	if *payloadStr != "" {
		var payload map[string]any
		if err := json.Unmarshal([]byte(*payloadStr), &payload); err != nil {
			return fmt.Errorf("parsing payload: %w", err)
		}
		if err := coll.Update(*id, payload); err != nil {
			return fmt.Errorf("updating payload: %w", err)
		}
	}

	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"status": "updated", "id": *id, "collection": collName})
		return nil
	}
	fmt.Printf("Updated vector with ID: %d\n", *id)
	return nil
}

func cmdDeleteWhere(args []string) error {
	fs := flag.NewFlagSet("delete-where", flag.ExitOnError)
	filterStr := fs.String("filter", "", "Filter expression, e.g., 'type=code'")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite delete-where [options] <file> <collection>")
		fmt.Println("\nDelete all vectors matching a filter.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite delete-where data.veclite embeddings --filter='type=code'")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("missing required arguments: file and collection")
	}
	if *filterStr == "" {
		return fmt.Errorf("--filter is required")
	}

	path := fs.Arg(0)
	collName := fs.Arg(1)

	filter := parseFilter(*filterStr)
	if filter == nil {
		return fmt.Errorf("invalid filter expression")
	}

	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	coll, err := db.GetCollection(collName)
	if err != nil {
		return fmt.Errorf("getting collection: %w", err)
	}

	deleted, err := coll.DeleteWhere(filter)
	if err != nil {
		return fmt.Errorf("deleting: %w", err)
	}

	if err := db.Sync(); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"status": "deleted", "deleted": deleted, "collection": collName})
		return nil
	}
	fmt.Printf("Deleted %d vectors\n", deleted)
	return nil
}

func cmdFind(args []string) error {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	filterStr := fs.String("filter", "", "Filter expression, e.g., 'type=code'")
	limit := fs.Int("limit", 0, "Maximum number of results (0 = all)")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite find [options] <file> <collection>")
		fmt.Println("\nFind records by filter (no vector needed).")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite find data.veclite embeddings --filter='type=code'")
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

	var filters []veclite.Filter
	if *filterStr != "" {
		f := parseFilter(*filterStr)
		if f != nil {
			filters = append(filters, f)
		}
	}

	records, err := coll.Find(filters...)
	if err != nil {
		return fmt.Errorf("finding records: %w", err)
	}

	if *limit > 0 && len(records) > *limit {
		records = records[:*limit]
	}

	if *jsonOutput {
		type recordOutput struct {
			ID      uint64         `json:"id"`
			Payload map[string]any `json:"payload,omitempty"`
			Content string         `json:"content,omitempty"`
		}
		output := make([]recordOutput, len(records))
		for i, r := range records {
			output[i] = recordOutput{
				ID:      r.ID,
				Payload: r.Payload,
				Content: r.Content,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(output)
		return nil
	}

	if len(records) == 0 {
		fmt.Println("No records found")
		return nil
	}

	fmt.Printf("Found %d records:\n", len(records))
	for i, r := range records {
		fmt.Printf("%d. ID=%d", i+1, r.ID)
		if r.Payload != nil {
			payloadJSON, _ := json.Marshal(r.Payload)
			fmt.Printf(", payload=%s", payloadJSON)
		}
		if r.Content != "" {
			if len(r.Content) > 80 {
				fmt.Printf(", content=%q...", r.Content[:80])
			} else {
				fmt.Printf(", content=%q", r.Content)
			}
		}
		fmt.Println()
	}
	return nil
}
