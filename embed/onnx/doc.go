//go:build onnx
// +build onnx

// Package onnx provides a local ONNX-based embedding system for veclite.
//
// This package implements the veclite.Embedder interface using ONNX Runtime
// for inference and HuggingFace tokenizers for text processing. It supports
// the all-MiniLM-L6-v2 model by default, providing 384-dimensional embeddings.
//
// # Features
//
//   - Zero external API dependencies (no Ollama/OpenAI required)
//   - Offline capability - text never leaves your machine
//   - Low latency (~5ms per embedding)
//   - Batch embedding support for efficiency
//
// # Requirements
//
// This package requires:
//   - ONNX Runtime shared library installed on your system
//   - Model files (model.onnx and tokenizer.json)
//
// Build with the "onnx" tag:
//
//	go build -tags onnx ./...
//
// # Model Download
//
// Use DownloadMiniLM to automatically download the all-MiniLM-L6-v2 model:
//
//	err := onnx.DownloadMiniLM("./models")
//
// Or manually download from HuggingFace:
// https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2
//
// # Example Usage
//
//	embedder, err := onnx.NewMiniLM("./models")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer embedder.Close()
//
//	// Use with veclite
//	db, _ := veclite.Open("data.veclite")
//	coll, _ := db.CreateCollection("docs",
//	    veclite.WithDimension(384),
//	    veclite.WithEmbedder(embedder),
//	)
//
//	id, _ := coll.InsertText("Hello world", nil)
//	results, _ := coll.SearchText("greeting", veclite.TopK(5))
package onnx
