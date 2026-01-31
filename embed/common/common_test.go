package common

import (
	"math"
	"testing"
)

func TestL2Normalize(t *testing.T) {
	tests := []struct {
		name  string
		input []float32
	}{
		{
			name:  "simple vector",
			input: []float32{3.0, 4.0},
		},
		{
			name:  "uniform vector",
			input: []float32{1.0, 1.0, 1.0, 1.0},
		},
		{
			name:  "mixed signs",
			input: []float32{-1.0, 2.0, -3.0, 4.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := L2Normalize(tt.input)

			// Calculate magnitude
			var mag float64
			for _, v := range result {
				mag += float64(v) * float64(v)
			}
			mag = math.Sqrt(mag)

			// Should be approximately 1.0
			if math.Abs(mag-1.0) > 1e-6 {
				t.Errorf("expected magnitude 1.0, got %f", mag)
			}
		})
	}
}

func TestL2NormalizeZeroVector(t *testing.T) {
	input := []float32{0.0, 0.0, 0.0}
	result := L2Normalize(input)

	// Should return same vector (avoid division by zero)
	for i, v := range result {
		if v != 0.0 {
			t.Errorf("expected 0.0 at index %d, got %f", i, v)
		}
	}
}

func TestL2NormalizeInPlace(t *testing.T) {
	input := []float32{3.0, 4.0}
	L2NormalizeInPlace(input)

	// Calculate magnitude
	var mag float64
	for _, v := range input {
		mag += float64(v) * float64(v)
	}
	mag = math.Sqrt(mag)

	if math.Abs(mag-1.0) > 1e-6 {
		t.Errorf("expected magnitude 1.0, got %f", mag)
	}
}

func TestFloat64ToFloat32(t *testing.T) {
	input := []float64{1.5, 2.5, 3.5}
	result := Float64ToFloat32(input)

	if len(result) != len(input) {
		t.Errorf("expected length %d, got %d", len(input), len(result))
	}

	for i, v := range result {
		expected := float32(input[i])
		if v != expected {
			t.Errorf("index %d: expected %f, got %f", i, expected, v)
		}
	}
}

func TestAPIError(t *testing.T) {
	err := NewAPIError("test", 401, "unauthorized")
	if err.StatusCode != 401 {
		t.Errorf("expected StatusCode 401, got %d", err.StatusCode)
	}
	if err.Provider != "test" {
		t.Errorf("expected Provider 'test', got %q", err.Provider)
	}
	if err.Message != "unauthorized" {
		t.Errorf("expected Message 'unauthorized', got %q", err.Message)
	}
	if err.Retryable {
		t.Error("expected Retryable false for 401")
	}
}

func TestAPIErrorRetryable(t *testing.T) {
	tests := []struct {
		code      int
		retryable bool
	}{
		{200, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
	}

	for _, tt := range tests {
		err := NewAPIError("test", tt.code, "test")
		if err.Retryable != tt.retryable {
			t.Errorf("status %d: expected Retryable %v, got %v", tt.code, tt.retryable, err.Retryable)
		}
	}
}

func TestAPIErrorString(t *testing.T) {
	err := NewAPIError("openai", 401, "Invalid API key")
	expected := "openai API error (status 401): Invalid API key"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
}
