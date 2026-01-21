// Command veclite provides a CLI for inspecting VecLite databases.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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

	cmd := os.Args[1]
	switch cmd {
	case "version":
		cmdVersion()
	case "info":
		cmdInfo(os.Args[2:])
	case "collections":
		cmdCollections(os.Args[2:])
	case "stats":
		cmdStats(os.Args[2:])
	case "dump":
		cmdDump(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`veclite - VecLite database CLI

Usage:
  veclite <command> [arguments]

Commands:
  version              Show version information
  info <file>          Show database information
  collections <file>   List all collections
  stats <file>         Show detailed statistics
  dump <file>          Dump database contents as JSON
  help                 Show this help message

Examples:
  veclite version
  veclite info data.veclite
  veclite stats data.veclite
  veclite collections data.veclite
  veclite dump data.veclite`)
}

func cmdVersion() {
	fmt.Printf("veclite version %s (library %s)\n", version, veclite.Version)
	if commit != "none" {
		fmt.Printf("commit: %s\n", commit)
	}
}

func cmdInfo(args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: veclite info <file>")
		fmt.Println("\nShow database information.")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)
	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	stats := db.Stats()
	fmt.Printf("Database: %s\n", path)
	fmt.Printf("Collections: %d\n", stats.Collections)
	fmt.Printf("Total Records: %d\n", stats.TotalRecords)
}

func cmdCollections(args []string) {
	fs := flag.NewFlagSet("collections", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: veclite collections <file>")
		fmt.Println("\nList all collections in the database.")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)
	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	collections := db.Collections()
	if len(collections) == 0 {
		fmt.Println("No collections")
		return
	}

	for _, name := range collections {
		coll, _ := db.GetCollection(name)
		stats := coll.Stats()
		fmt.Printf("%s: %d records, dimension=%d, distance=%s\n",
			name, stats.Count, stats.Dimension, stats.DistanceType)
	}
}

func cmdStats(args []string) {
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
		os.Exit(1)
	}

	path := fs.Arg(0)
	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	stats := db.Stats()

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(stats)
		return
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
		}
	}
}

func cmdDump(args []string) {
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
		os.Exit(1)
	}

	path := fs.Arg(0)
	db, err := veclite.Open(path, veclite.WithReadOnly(true))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
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
}
