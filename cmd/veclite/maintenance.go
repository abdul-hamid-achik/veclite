package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/veclite"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

func cmdCompact(args []string) error {
	fs := flag.NewFlagSet("compact", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite compact [options] <file>")
		fmt.Println("\nCompact the database to reclaim space from deleted records.")
		fmt.Println("This rewrites the database file, removing any soft-deleted data.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}

	path := fs.Arg(0)

	// Get stats before compaction
	db, err := veclite.Open(path, veclite.WithReadOnly(true), veclite.WithSharedRead(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	statsBefore := db.Stats()
	_ = db.Close()

	// Get file size before
	fileBefore, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("getting file stats: %w", err)
	}
	sizeBefore := fileBefore.Size()

	// Open for write and sync to compact
	db, err = veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database for compaction: %w", err)
	}

	if err := db.Sync(); err != nil {
		_ = db.Close()
		return fmt.Errorf("syncing database: %w", err)
	}

	_ = db.Close()

	// Get file size after
	fileAfter, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("getting file stats after compaction: %w", err)
	}
	sizeAfter := fileAfter.Size()

	if *jsonOutput {
		result := map[string]any{
			"status":        "compacted",
			"size_before":   sizeBefore,
			"size_after":    sizeAfter,
			"bytes_saved":   sizeBefore - sizeAfter,
			"collections":   statsBefore.Collections,
			"total_records": statsBefore.TotalRecords,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	fmt.Printf("Database compacted: %s\n", path)
	fmt.Printf("Size before: %s\n", formatBytes(sizeBefore))
	fmt.Printf("Size after:  %s\n", formatBytes(sizeAfter))
	fmt.Printf("Saved:       %s\n", formatBytes(sizeBefore-sizeAfter))
	return nil
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite validate [options] <file>")
		fmt.Println("\nValidate database integrity and check for corruption.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}

	path := fs.Arg(0)

	// Check file exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("database file not found")
		}
		return fmt.Errorf("accessing file: %w", err)
	}

	issues := make([]string, 0)
	warnings := make([]string, 0)

	// Try to open and read the database
	db, err := veclite.Open(path, veclite.WithReadOnly(true), veclite.WithSharedRead(true))
	if err != nil {
		issues = append(issues, fmt.Sprintf("Failed to open database: %v", err))
		outputValidation(*jsonOutput, path, fileInfo.Size(), issues, warnings, false)
		return fmt.Errorf("database validation failed")
	}
	defer func() { _ = db.Close() }()

	// Validate each collection
	for _, collName := range db.Collections() {
		coll, err := db.GetCollection(collName)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Collection '%s': failed to get: %v", collName, err))
			continue
		}

		collStats := coll.Stats()

		// Check for dimension consistency
		records := coll.All()
		if len(records) > 0 {
			expectedDim := len(records[0].Vector)
			for _, r := range records[1:] {
				if len(r.Vector) != expectedDim {
					issues = append(issues, fmt.Sprintf("Collection '%s': record %d has dimension %d, expected %d",
						collName, r.ID, len(r.Vector), expectedDim))
				}
			}

			// Check if stats dimension matches actual
			if collStats.Dimension != 0 && collStats.Dimension != expectedDim {
				warnings = append(warnings, fmt.Sprintf("Collection '%s': stats dimension (%d) doesn't match actual (%d)",
					collName, collStats.Dimension, expectedDim))
			}
		}

		// Check for zero vectors (potential issue)
		for _, r := range records {
			allZero := true
			for _, v := range r.Vector {
				if v != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				warnings = append(warnings, fmt.Sprintf("Collection '%s': record %d has all-zero vector", collName, r.ID))
			}
		}
	}

	valid := len(issues) == 0
	outputValidation(*jsonOutput, path, fileInfo.Size(), issues, warnings, valid)

	if !valid {
		return fmt.Errorf("database validation failed")
	}
	return nil
}

func outputValidation(jsonOutput bool, path string, size int64, issues, warnings []string, valid bool) {
	if jsonOutput {
		result := map[string]any{
			"path":     path,
			"size":     size,
			"valid":    valid,
			"issues":   issues,
			"warnings": warnings,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}

	fmt.Printf("Database: %s\n", path)
	fmt.Printf("Size:     %s\n", formatBytes(size))
	fmt.Println()

	if len(issues) > 0 {
		fmt.Println("ISSUES FOUND:")
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
		fmt.Println()
	}

	if len(warnings) > 0 {
		fmt.Println("WARNINGS:")
		for _, warning := range warnings {
			fmt.Printf("  - %s\n", warning)
		}
		fmt.Println()
	}

	if valid {
		fmt.Println("Result: VALID")
	} else {
		fmt.Println("Result: INVALID")
	}
}

func cmdBenchmark(args []string) error {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)
	collection := fs.String("collection", "", "Collection to benchmark (required)")
	queries := fs.Int("queries", 100, "Number of search queries to run")
	topK := fs.Int("top-k", 10, "Top-K results per query")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = func() {
		fmt.Println("Usage: veclite benchmark [options] <file>")
		fmt.Println("\nRun search performance benchmark on a collection.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  veclite benchmark data.veclite --collection=embeddings")
		fmt.Println("  veclite benchmark data.veclite --collection=embeddings --queries=1000 --top-k=20")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}

	if *collection == "" {
		return fmt.Errorf("--collection is required")
	}

	path := fs.Arg(0)

	db, err := veclite.Open(path, veclite.WithReadOnly(true), veclite.WithSharedRead(true))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	coll, err := db.GetCollection(*collection)
	if err != nil {
		return fmt.Errorf("getting collection: %w", err)
	}

	stats := coll.Stats()
	if stats.Count == 0 {
		return fmt.Errorf("collection is empty")
	}

	// Get all records to use as query vectors
	records := coll.All()
	dimension := stats.Dimension
	if dimension == 0 && len(records) > 0 {
		dimension = len(records[0].Vector)
	}

	if !*jsonOutput {
		fmt.Printf("Benchmarking: %s\n", *collection)
		fmt.Printf("Records: %d, Dimension: %d, Index: %s\n", stats.Count, dimension, stats.IndexType)
		fmt.Printf("Queries: %d, Top-K: %d\n", *queries, *topK)
		fmt.Println()
		fmt.Println("Running benchmark...")
	}

	// Generate random query vectors based on existing data
	queryVectors := make([][]float32, *queries)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < *queries; i++ {
		// Use a random existing vector as base
		base := records[rng.Intn(len(records))].Vector
		// Add some noise
		queryVectors[i] = make([]float32, len(base))
		for j, v := range base {
			queryVectors[i][j] = v + float32(rng.NormFloat64()*0.1)
		}
	}

	// Run benchmark
	start := time.Now()
	totalResults := 0
	for _, q := range queryVectors {
		results, err := coll.Search(q, veclite.TopK(*topK))
		if err != nil {
			return fmt.Errorf("during search: %w", err)
		}
		totalResults += len(results)
	}
	duration := time.Since(start)

	// Calculate metrics
	avgLatency := duration / time.Duration(*queries)
	qps := float64(*queries) / duration.Seconds()

	if *jsonOutput {
		result := map[string]any{
			"collection":     *collection,
			"record_count":   stats.Count,
			"dimension":      dimension,
			"index_type":     stats.IndexType,
			"queries":        *queries,
			"top_k":          *topK,
			"total_time_ms":  float64(duration.Microseconds()) / 1000.0,
			"avg_latency_ms": float64(avgLatency.Microseconds()) / 1000.0,
			"qps":            qps,
			"total_results":  totalResults,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return nil
	}

	fmt.Println()
	fmt.Println("Results:")
	fmt.Printf("  Total time:    %s\n", duration.Round(time.Millisecond))
	fmt.Printf("  Avg latency:   %s\n", avgLatency.Round(time.Microsecond))
	fmt.Printf("  Queries/sec:   %.2f\n", qps)
	fmt.Printf("  Total results: %d\n", totalResults)
	return nil
}

// formatBytes formats bytes in human-readable format.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func cmdUnlock(args []string) error {
	fs := flag.NewFlagSet("unlock", flag.ExitOnError)
	force := fs.Bool("force", false, "Remove the lock even if the holding process is alive")
	fs.Usage = func() {
		fmt.Println("Usage: veclite unlock [options] <file>")
		fmt.Println("\nInspect and remove an orphaned database lock file.")
		fmt.Println("Locks held by dead processes are removed automatically.")
		fmt.Println("Locks held by live processes are reported but not removed")
		fmt.Println("unless --force is given.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}
	return unlockDB(fs.Arg(0), *force, os.Stdout)
}

// unlockDB inspects the lock file for dbPath and removes it when safe (or
// when forced). Extracted from cmdUnlock so it can be unit-tested without
// exec'ing the binary.
func unlockDB(dbPath string, force bool, out io.Writer) error {
	if !storage.LockFileExists(dbPath) {
		fmt.Fprintf(out, "no lock file at %s.lock\n", dbPath)
		return nil
	}

	pid := storage.ReadLockPID(dbPath)
	info := storage.ReadLockInfo(dbPath)

	if pid <= 0 {
		// Unparseable lock file — treat as stale garbage.
		if err := storage.RemoveLockFile(dbPath); err != nil {
			return fmt.Errorf("removing unparseable lock file: %w", err)
		}
		fmt.Fprintf(out, "removed unparseable lock file at %s.lock\n", dbPath)
		return nil
	}

	if !storage.IsProcessAlive(pid) {
		if err := storage.RemoveLockFile(dbPath); err != nil {
			return fmt.Errorf("removing stale lock: %w", err)
		}
		fmt.Fprintf(out, "removed stale lock (PID %d, dead)\n", pid)
		return nil
	}

	// Holder is alive.
	cmdline := processCommand(pid)
	fmt.Fprintf(out, "lock held by live process: %s\n", info)
	if cmdline != "" {
		fmt.Fprintf(out, "  command: %s\n", cmdline)
	}
	if !force {
		fmt.Fprintln(out, "refusing to remove a live process's lock.")
		fmt.Fprintf(out, "kill the process first (kill %d), or re-run with --force if you are certain it is stuck.\n", pid)
		return fmt.Errorf("lock held by live PID %d", pid)
	}

	if err := storage.RemoveLockFile(dbPath); err != nil {
		return fmt.Errorf("force-removing lock: %w", err)
	}
	fmt.Fprintf(out, "WARNING: force-removed lock held by live PID %d — if that process writes again, the database may be corrupted\n", pid)
	return nil
}

// processCommand returns the command line of a process, best-effort.
func processCommand(pid int) string {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
