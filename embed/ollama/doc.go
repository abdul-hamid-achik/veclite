// Package ollama provides an Ollama-based embedding system for veclite.
//
// This package implements the veclite.Embedder interface using Ollama's
// local embedding API. It supports any embedding model available in Ollama,
// including nomic-embed-text, mxbai-embed-large, and all-minilm.
//
// # Features
//
//   - Local inference with no API key required
//   - Privacy-preserving - data never leaves your machine
//   - Support for any Ollama embedding model
//   - Automatic dimension detection on first call
//
// # Popular Models
//
// | Model              | Dimensions | Notes              |
// |--------------------|------------|--------------------|
// | nomic-embed-text   | 768        | Good general purpose |
// | mxbai-embed-large  | 1024       | High quality       |
// | all-minilm         | 384        | Fast, lightweight  |
//
// # Requirements
//
// Ollama must be running locally (default: http://localhost:11434).
// Pull your desired model first:
//
//	ollama pull nomic-embed-text
//
// # Example Usage
//
//	embedder, err := ollama.NewEmbedder(
//	    ollama.WithModel("nomic-embed-text"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer embedder.Close()
//
//	// Use with veclite
//	db, _ := veclite.Open("data.veclite")
//	coll, _ := db.CreateCollection("docs",
//	    veclite.WithDimension(embedder.Dimension()),
//	    veclite.WithEmbedder(embedder),
//	)
//
//	id, _ := coll.InsertText("Hello world", nil)
//	results, _ := coll.SearchText("greeting", veclite.TopK(5))
package ollama
