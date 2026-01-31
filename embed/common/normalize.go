package common

import "math"

// L2Normalize normalizes a vector to unit length using L2 (Euclidean) norm.
// Returns the original vector if it has zero magnitude to avoid division by zero.
func L2Normalize(v []float32) []float32 {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return v
	}
	norm := float32(1.0 / math.Sqrt(float64(sum)))
	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = x * norm
	}
	return result
}

// L2NormalizeInPlace normalizes a vector in place to unit length.
// More efficient than L2Normalize when the original slice can be modified.
func L2NormalizeInPlace(v []float32) {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return
	}
	norm := float32(1.0 / math.Sqrt(float64(sum)))
	for i := range v {
		v[i] *= norm
	}
}

// Float64ToFloat32 converts a slice of float64 to float32.
// This is commonly needed when parsing JSON API responses.
func Float64ToFloat32(v []float64) []float32 {
	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = float32(x)
	}
	return result
}
