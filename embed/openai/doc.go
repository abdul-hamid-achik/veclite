// Package openai provides an OpenAI-based embedding system for veclite.
//
// This package implements the veclite.Embedder interface using OpenAI's
// embedding API. It supports the text-embedding-3-small and text-embedding-3-large
// models, as well as the legacy text-embedding-ada-002.
//
// # Features
//
//   - High-quality embeddings from OpenAI's models
//   - Batch embedding support for efficiency
//   - Automatic retry with exponential backoff for rate limits
//   - Configurable model dimensions (for text-embedding-3-* models)
//
// # Models
//
// | Model                    | Dimensions         | Notes                    |
// |--------------------------|--------------------|-----------------------   |
// | text-embedding-3-small   | 1536 (default)     | Cheapest, good quality   |
// | text-embedding-3-large   | 3072 (default)     | Best quality             |
// | text-embedding-ada-002   | 1536               | Legacy                   |
//
// # Requirements
//
// An OpenAI API key is required. Set it via:
//   - The OPENAI_API_KEY environment variable
//   - The WithAPIKey option
//   - Configuration in veclite.yaml
//
// # Example Usage
//
//	embedder, err := openai.NewEmbedder(
//	    openai.WithAPIKey("sk-..."),
//	    openai.WithModel("text-embedding-3-small"),
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
package openai
