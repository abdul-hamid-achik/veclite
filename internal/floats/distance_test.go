package floats

import (
	"math"
	"testing"
)

func TestCosine(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
		epsilon  float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
			epsilon:  0.0001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
			epsilon:  0.0001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
			epsilon:  0.0001,
		},
		{
			name:     "45 degree angle",
			a:        []float32{1, 0},
			b:        []float32{1, 1},
			expected: float32(1 / math.Sqrt(2)),
			epsilon:  0.0001,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0,
			epsilon:  0.0001,
		},
		{
			name:     "mismatched lengths",
			a:        []float32{1, 2},
			b:        []float32{1, 2, 3},
			expected: 0,
			epsilon:  0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Cosine(tt.a, tt.b)
			if math.Abs(float64(result-tt.expected)) > float64(tt.epsilon) {
				t.Errorf("Cosine(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestDot(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
	}{
		{
			name:     "basic dot product",
			a:        []float32{1, 2, 3},
			b:        []float32{4, 5, 6},
			expected: 32, // 1*4 + 2*5 + 3*6 = 4 + 10 + 18 = 32
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0},
			b:        []float32{0, 1},
			expected: 0,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Dot(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Dot(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestEuclidean(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
		epsilon  float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 2, 3},
			expected: 0,
			epsilon:  0.0001,
		},
		{
			name:     "unit distance",
			a:        []float32{0, 0},
			b:        []float32{1, 0},
			expected: 1,
			epsilon:  0.0001,
		},
		{
			name:     "3-4-5 triangle",
			a:        []float32{0, 0},
			b:        []float32{3, 4},
			expected: 5,
			epsilon:  0.0001,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0,
			epsilon:  0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Euclidean(tt.a, tt.b)
			if math.Abs(float64(result-tt.expected)) > float64(tt.epsilon) {
				t.Errorf("Euclidean(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestMagnitude(t *testing.T) {
	tests := []struct {
		name     string
		v        []float32
		expected float32
		epsilon  float32
	}{
		{
			name:     "unit vector",
			v:        []float32{1, 0, 0},
			expected: 1,
			epsilon:  0.0001,
		},
		{
			name:     "3-4-5 vector",
			v:        []float32{3, 4},
			expected: 5,
			epsilon:  0.0001,
		},
		{
			name:     "empty vector",
			v:        []float32{},
			expected: 0,
			epsilon:  0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Magnitude(tt.v)
			if math.Abs(float64(result-tt.expected)) > float64(tt.epsilon) {
				t.Errorf("Magnitude(%v) = %v, want %v", tt.v, result, tt.expected)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	v := []float32{3, 4}
	Normalize(v)

	expected := []float32{0.6, 0.8}
	for i := range v {
		if math.Abs(float64(v[i]-expected[i])) > 0.0001 {
			t.Errorf("Normalize: v[%d] = %v, want %v", i, v[i], expected[i])
		}
	}

	// Verify magnitude is 1
	mag := Magnitude(v)
	if math.Abs(float64(mag-1)) > 0.0001 {
		t.Errorf("Magnitude after normalize = %v, want 1", mag)
	}
}

func TestNormalizeCopy(t *testing.T) {
	original := []float32{3, 4}
	result := NormalizeCopy(original)

	// Original should be unchanged
	if original[0] != 3 || original[1] != 4 {
		t.Error("NormalizeCopy modified the original vector")
	}

	// Result should be normalized
	expected := []float32{0.6, 0.8}
	for i := range result {
		if math.Abs(float64(result[i]-expected[i])) > 0.0001 {
			t.Errorf("NormalizeCopy: result[%d] = %v, want %v", i, result[i], expected[i])
		}
	}
}

func TestGetDistanceFunc(t *testing.T) {
	tests := []struct {
		distType DistanceType
		a, b     []float32
	}{
		{DistanceCosine, []float32{1, 0}, []float32{1, 0}},
		{DistanceDot, []float32{1, 2}, []float32{3, 4}},
		{DistanceEuclidean, []float32{0, 0}, []float32{3, 4}},
	}

	for _, tt := range tests {
		t.Run(string(tt.distType), func(t *testing.T) {
			fn := GetDistanceFunc(tt.distType)
			if fn == nil {
				t.Errorf("GetDistanceFunc(%v) returned nil", tt.distType)
			}
			// Just verify it doesn't panic
			fn(tt.a, tt.b)
		})
	}
}

func TestIsHigherBetter(t *testing.T) {
	tests := []struct {
		distType DistanceType
		expected bool
	}{
		{DistanceCosine, true},
		{DistanceDot, true},
		{DistanceEuclidean, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.distType), func(t *testing.T) {
			result := IsHigherBetter(tt.distType)
			if result != tt.expected {
				t.Errorf("IsHigherBetter(%v) = %v, want %v", tt.distType, result, tt.expected)
			}
		})
	}
}

// Benchmarks

func BenchmarkCosine(b *testing.B) {
	v1 := make([]float32, 384)
	v2 := make([]float32, 384)
	for i := range v1 {
		v1[i] = float32(i) / 384
		v2[i] = float32(i+1) / 384
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Cosine(v1, v2)
	}
}

func BenchmarkDot(b *testing.B) {
	v1 := make([]float32, 384)
	v2 := make([]float32, 384)
	for i := range v1 {
		v1[i] = float32(i) / 384
		v2[i] = float32(i+1) / 384
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Dot(v1, v2)
	}
}

func BenchmarkEuclidean(b *testing.B) {
	v1 := make([]float32, 384)
	v2 := make([]float32, 384)
	for i := range v1 {
		v1[i] = float32(i) / 384
		v2[i] = float32(i+1) / 384
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Euclidean(v1, v2)
	}
}
