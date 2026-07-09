//go:build onnx
// +build onnx

package onnx

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/daulet/tokenizers"
	ort "github.com/yalue/onnxruntime_go"

	"github.com/abdul-hamid-achik/veclite/embed/common"
)

const (
	// MiniLMDimension is the embedding dimension for all-MiniLM-L6-v2.
	MiniLMDimension = 384

	// MiniLMModel is the model name of the bundled sentence-transformers model.
	MiniLMModel = "all-MiniLM-L6-v2"

	// DefaultMaxLength is the maximum token sequence length.
	DefaultMaxLength = 256

	// HuggingFace model URLs
	miniLMModelURL     = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
	miniLMTokenizerURL = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"
)

// miniLMDimension resolves the MiniLM dimension through the shared registry
// in embed/common, falling back to the hardcoded MiniLMDimension constant.
func miniLMDimension() int {
	if dim, ok := common.KnownModelDimensions(MiniLMModel); ok {
		return dim
	}
	return MiniLMDimension
}

// Embedder implements veclite.Embedder using ONNX Runtime for local inference.
type Embedder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *tokenizers.Tokenizer
	dim       int
	maxLen    int
	mu        sync.Mutex
}

// SetLibraryPath sets the path to the ONNX Runtime shared library.
// This must be called before creating any embedder if the library is not
// in the default search path. On macOS with Homebrew, use:
//
//	onnx.SetLibraryPath("/opt/homebrew/lib/libonnxruntime.dylib")
func SetLibraryPath(path string) {
	ort.SetSharedLibraryPath(path)
}

func init() {
	// Try common library paths if not already set
	paths := []string{
		"/opt/homebrew/lib/libonnxruntime.dylib", // macOS ARM (Homebrew)
		"/usr/local/lib/libonnxruntime.dylib",    // macOS Intel (Homebrew)
		"/usr/lib/libonnxruntime.so",             // Linux system
		"/usr/local/lib/libonnxruntime.so",       // Linux local
		os.Getenv("ONNXRUNTIME_LIB"),             // User-specified
	}

	for _, p := range paths {
		if p != "" {
			if _, err := os.Stat(p); err == nil {
				ort.SetSharedLibraryPath(p)
				break
			}
		}
	}
}

// NewEmbedder creates an ONNX embedder from model and tokenizer paths.
func NewEmbedder(modelPath, tokenizerPath string, opts ...Option) (*Embedder, error) {
	cfg := &config{
		maxLen: DefaultMaxLength,
		dim:    MiniLMDimension,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Initialize ONNX Runtime (only if not already initialized)
	if !ort.IsInitialized() {
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("onnx: failed to initialize runtime: %w", err)
		}
	}

	// Load tokenizer
	tk, err := tokenizers.FromFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("onnx: failed to load tokenizer: %w", err)
	}

	// Create ONNX session with dynamic shapes
	// Input/output names for sentence-transformers models
	inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputNames := []string{"last_hidden_state"}

	sessionOpts, err := ort.NewSessionOptions()
	if err != nil {
		tk.Close()
		return nil, fmt.Errorf("onnx: failed to create session options: %w", err)
	}
	defer sessionOpts.Destroy()

	session, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, sessionOpts)
	if err != nil {
		tk.Close()
		return nil, fmt.Errorf("onnx: failed to create session: %w", err)
	}

	return &Embedder{
		session:   session,
		tokenizer: tk,
		dim:       cfg.dim,
		maxLen:    cfg.maxLen,
	}, nil
}

// NewMiniLM creates an embedder using the all-MiniLM-L6-v2 model.
// modelDir should contain model.onnx and tokenizer.json files.
func NewMiniLM(modelDir string) (*Embedder, error) {
	modelPath := filepath.Join(modelDir, "model.onnx")
	tokenizerPath := filepath.Join(modelDir, "tokenizer.json")

	// Check if files exist
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("onnx: model.onnx not found in %s (use DownloadMiniLM to download)", modelDir)
	}
	if _, err := os.Stat(tokenizerPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("onnx: tokenizer.json not found in %s (use DownloadMiniLM to download)", modelDir)
	}

	return NewEmbedder(modelPath, tokenizerPath,
		WithDimension(miniLMDimension()),
		WithMaxLength(DefaultMaxLength),
	)
}

// Embed converts a single text into a vector embedding.
func (e *Embedder) Embed(text string) ([]float32, error) {
	results, err := e.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// ErrClosed is returned when operations are attempted on a closed embedder.
var ErrClosed = fmt.Errorf("onnx: embedder is closed")

// EmbedBatch converts multiple texts into vector embeddings.
func (e *Embedder) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.session == nil || e.tokenizer == nil {
		return nil, ErrClosed
	}

	batchSize := len(texts)

	// Tokenize all texts with attention mask and type IDs
	encodings := make([]tokenizers.Encoding, batchSize)
	maxSeqLen := 0
	for i, text := range texts {
		enc := e.tokenizer.EncodeWithOptions(text, true,
			tokenizers.WithReturnAttentionMask(),
			tokenizers.WithReturnTypeIDs(),
		)
		encodings[i] = enc

		if len(enc.IDs) > maxSeqLen {
			maxSeqLen = len(enc.IDs)
		}
	}

	// Truncate to max length
	if maxSeqLen > e.maxLen {
		maxSeqLen = e.maxLen
	}

	// Prepare input tensors (padded to same length)
	inputIDs := make([]int64, batchSize*maxSeqLen)
	attentionMask := make([]int64, batchSize*maxSeqLen)
	tokenTypeIDs := make([]int64, batchSize*maxSeqLen) // zeros for single-sentence

	for i, enc := range encodings {
		seqLen := len(enc.IDs)
		if seqLen > maxSeqLen {
			seqLen = maxSeqLen
		}

		for j := 0; j < seqLen; j++ {
			idx := i*maxSeqLen + j
			inputIDs[idx] = int64(enc.IDs[j])
			if j < len(enc.AttentionMask) {
				attentionMask[idx] = int64(enc.AttentionMask[j])
			} else {
				attentionMask[idx] = 1 // default to attending
			}
			if j < len(enc.TypeIDs) {
				tokenTypeIDs[idx] = int64(enc.TypeIDs[j])
			}
		}
		// Remaining positions are already 0 (padding)
	}

	// Create input tensors
	inputShape := ort.Shape{int64(batchSize), int64(maxSeqLen)}

	inputIDsTensor, err := ort.NewTensor(inputShape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("onnx: failed to create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	attentionMaskTensor, err := ort.NewTensor(inputShape, attentionMask)
	if err != nil {
		return nil, fmt.Errorf("onnx: failed to create attention_mask tensor: %w", err)
	}
	defer attentionMaskTensor.Destroy()

	tokenTypeIDsTensor, err := ort.NewTensor(inputShape, tokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("onnx: failed to create token_type_ids tensor: %w", err)
	}
	defer tokenTypeIDsTensor.Destroy()

	// Create output tensor
	outputShape := ort.Shape{int64(batchSize), int64(maxSeqLen), int64(e.dim)}
	outputData := make([]float32, batchSize*maxSeqLen*e.dim)
	outputTensor, err := ort.NewTensor(outputShape, outputData)
	if err != nil {
		return nil, fmt.Errorf("onnx: failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// Run inference
	err = e.session.Run(
		[]ort.Value{inputIDsTensor, attentionMaskTensor, tokenTypeIDsTensor},
		[]ort.Value{outputTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("onnx: inference failed: %w", err)
	}

	// Extract embeddings using mean pooling over non-padded tokens
	results := make([][]float32, batchSize)
	for i := 0; i < batchSize; i++ {
		// Count non-padded tokens
		enc := encodings[i]
		seqLen := len(enc.IDs)
		if seqLen > maxSeqLen {
			seqLen = maxSeqLen
		}

		// Mean pooling over valid tokens
		embedding := make([]float32, e.dim)
		for j := 0; j < seqLen; j++ {
			offset := (i*maxSeqLen + j) * e.dim
			for k := 0; k < e.dim; k++ {
				embedding[k] += outputData[offset+k]
			}
		}

		// Normalize by sequence length
		for k := 0; k < e.dim; k++ {
			embedding[k] /= float32(seqLen)
		}

		// L2 normalize the embedding
		results[i] = l2Normalize(embedding)
	}

	return results, nil
}

// Dimension returns the output vector dimension.
func (e *Embedder) Dimension() int {
	return e.dim
}

// Profile describes this embedder: provider "onnx", the bundled
// all-MiniLM-L6-v2 model, the configured dimension, cosine distance, and
// Normalize=true — EmbedBatch L2-normalizes every embedding after mean
// pooling (see l2Normalize).
func (e *Embedder) Profile() common.ProfileData {
	return common.ProfileData{
		Provider:  "onnx",
		Model:     MiniLMModel,
		Dimension: e.dim,
		Distance:  "cosine",
		Normalize: true,
	}
}

// Close releases ONNX Runtime resources.
func (e *Embedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.session != nil {
		if err := e.session.Destroy(); err != nil {
			return fmt.Errorf("onnx: failed to destroy session: %w", err)
		}
		e.session = nil
	}

	if e.tokenizer != nil {
		e.tokenizer.Close()
		e.tokenizer = nil
	}

	return nil
}

// DownloadMiniLM downloads the all-MiniLM-L6-v2 model to the specified directory.
func DownloadMiniLM(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("onnx: failed to create directory: %w", err)
	}

	// Download model.onnx
	modelPath := filepath.Join(dir, "model.onnx")
	if err := downloadFile(miniLMModelURL, modelPath); err != nil {
		return fmt.Errorf("onnx: failed to download model: %w", err)
	}

	// Download tokenizer.json
	tokenizerPath := filepath.Join(dir, "tokenizer.json")
	if err := downloadFile(miniLMTokenizerURL, tokenizerPath); err != nil {
		return fmt.Errorf("onnx: failed to download tokenizer: %w", err)
	}

	return nil
}

// DownloadMiniLMQuantized downloads the quantized (smaller) all-MiniLM-L6-v2 model.
// The quantized model is about 25MB vs 90MB for the full model.
func DownloadMiniLMQuantized(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("onnx: failed to create directory: %w", err)
	}

	// Quantized model URL
	quantizedURL := "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model_quantized.onnx"

	modelPath := filepath.Join(dir, "model.onnx")
	if err := downloadFile(quantizedURL, modelPath); err != nil {
		return fmt.Errorf("onnx: failed to download quantized model: %w", err)
	}

	tokenizerPath := filepath.Join(dir, "tokenizer.json")
	if err := downloadFile(miniLMTokenizerURL, tokenizerPath); err != nil {
		return fmt.Errorf("onnx: failed to download tokenizer: %w", err)
	}

	return nil
}

// config holds embedder configuration.
type config struct {
	maxLen int
	dim    int
}

// Option configures the embedder.
type Option func(*config)

// WithMaxLength sets the maximum token sequence length.
func WithMaxLength(maxLen int) Option {
	return func(c *config) {
		c.maxLen = maxLen
	}
}

// WithDimension sets the expected embedding dimension.
func WithDimension(dim int) Option {
	return func(c *config) {
		c.dim = dim
	}
}

// downloadFile downloads a URL to a local file.
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// l2Normalize normalizes a vector to unit length.
func l2Normalize(v []float32) []float32 {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return v
	}
	norm := float32(1.0) / sqrt32(sum)
	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = x * norm
	}
	return result
}

// sqrt32 computes the square root of a float32.
func sqrt32(x float32) float32 {
	return float32(sqrt64(float64(x)))
}

// sqrt64 computes the square root using Newton's method.
func sqrt64(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// Ensure Embedder implements the interface at compile time.
// This is verified against the veclite.Embedder interface when used.
var _ interface {
	Embed(string) ([]float32, error)
	EmbedBatch([]string) ([][]float32, error)
	Dimension() int
} = (*Embedder)(nil)

// extractZip extracts a zip file to a directory (unused but kept for potential future use).
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
