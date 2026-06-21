// Example http-client demonstrates using VecLite's HTTP API.
// Start the server first: veclite serve :memory: --port=8080
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const baseURL = "http://127.0.0.1:8080"

func main() {
	fmt.Println("VecLite HTTP Client Example")
	fmt.Println("Make sure the server is running: veclite serve data.veclite --port=8080")
	fmt.Println()

	// Health check
	fmt.Println("=== Health Check ===")
	resp := get("/health")
	fmt.Println(resp)

	// Create a collection
	fmt.Println("\n=== Create Collection ===")
	resp = post("/collections", map[string]any{
		"name":      "example",
		"dimension": 4,
		"distance":  "cosine",
	})
	fmt.Println(resp)

	// Insert vectors
	fmt.Println("\n=== Insert Vectors ===")
	resp = post("/collections/example/vectors", map[string]any{
		"vectors": [][]float64{
			{0.1, 0.2, 0.3, 0.4},
			{0.2, 0.3, 0.4, 0.5},
			{0.9, 0.8, 0.7, 0.6},
		},
		"payloads": []map[string]any{
			{"file": "main.go"},
			{"file": "utils.go"},
			{"file": "README.md"},
		},
	})
	fmt.Println(resp)

	// Search
	fmt.Println("\n=== Search ===")
	resp = post("/collections/example/search", map[string]any{
		"query": []float64{0.15, 0.25, 0.35, 0.45},
		"top_k": 2,
	})
	fmt.Println(resp)

	// Search with filter
	fmt.Println("\n=== Search with Filter ===")
	resp = post("/collections/example/search", map[string]any{
		"query": []float64{0.15, 0.25, 0.35, 0.45},
		"top_k": 2,
		"filters": []map[string]any{
			{"key": "file", "op": "suffix", "value": ".go"},
		},
	})
	fmt.Println(resp)

	// List vectors with pagination
	fmt.Println("\n=== List Vectors (offset=0, limit=2) ===")
	resp = get("/collections/example/vectors?offset=0&limit=2")
	fmt.Println(resp)

	// Get collection info
	fmt.Println("\n=== Collection Info ===")
	resp = get("/collections/example")
	fmt.Println(resp)
}

func get(path string) string {
	resp, err := http.Get(baseURL + path)
	if err != nil {
		log.Printf("GET %s failed: %v", path, err)
		return fmt.Sprintf("Error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return readBody(resp)
}

func post(path string, body any) string {
	b, _ := json.Marshal(body)
	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		log.Printf("POST %s failed: %v", path, err)
		return fmt.Sprintf("Error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return readBody(resp)
}

func readBody(resp *http.Response) string {
	body, _ := io.ReadAll(resp.Body)
	var out bytes.Buffer
	if json.Indent(&out, body, "", "  ") == nil {
		return out.String()
	}
	return string(body)
}
