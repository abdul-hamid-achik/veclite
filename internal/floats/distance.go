// Package floats provides optimized floating-point vector operations.
package floats

import "math"

// Cosine computes the cosine similarity between two vectors.
// Returns a value in [-1, 1] where 1 means identical direction,
// 0 means orthogonal, and -1 means opposite direction.
// Assumes vectors are non-zero length.
func Cosine(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / float32(math.Sqrt(float64(normA)*float64(normB)))
}

// Dot computes the dot product of two vectors.
func Dot(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// Euclidean computes the Euclidean (L2) distance between two vectors.
// Returns a non-negative value where 0 means identical vectors.
func Euclidean(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return float32(math.Sqrt(float64(sum)))
}

// EuclideanSquared computes the squared Euclidean distance.
// Useful for comparisons where the actual distance isn't needed.
func EuclideanSquared(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

// Magnitude computes the L2 norm (magnitude) of a vector.
func Magnitude(v []float32) float32 {
	if len(v) == 0 {
		return 0
	}

	var sum float32
	for _, x := range v {
		sum += x * x
	}
	return float32(math.Sqrt(float64(sum)))
}

// Normalize normalizes a vector in-place to unit length.
// If the vector has zero magnitude, it remains unchanged.
func Normalize(v []float32) {
	mag := Magnitude(v)
	if mag == 0 {
		return
	}
	invMag := 1.0 / mag
	for i := range v {
		v[i] *= invMag
	}
}

// NormalizeCopy returns a normalized copy of the vector.
// If the vector has zero magnitude, returns a zero vector.
func NormalizeCopy(v []float32) []float32 {
	result := make([]float32, len(v))
	mag := Magnitude(v)
	if mag == 0 {
		return result
	}
	invMag := 1.0 / mag
	for i := range v {
		result[i] = v[i] * invMag
	}
	return result
}

// DistanceFunc is a function type for computing distance/similarity between vectors.
type DistanceFunc func(a, b []float32) float32

// DistanceType represents the type of distance metric.
type DistanceType string

const (
	// DistanceCosine uses cosine similarity (higher = more similar).
	DistanceCosine DistanceType = "cosine"
	// DistanceDot uses dot product (higher = more similar).
	DistanceDot DistanceType = "dot"
	// DistanceEuclidean uses Euclidean distance (lower = more similar).
	DistanceEuclidean DistanceType = "euclidean"
	// DistanceEuclideanSquared uses squared Euclidean distance (lower = more similar).
	// Faster than Euclidean since it avoids the sqrt computation.
	DistanceEuclideanSquared DistanceType = "euclidean_squared"
)

// GetDistanceFunc returns the distance function for a given type.
func GetDistanceFunc(t DistanceType) DistanceFunc {
	switch t {
	case DistanceDot:
		return Dot
	case DistanceEuclidean:
		return Euclidean
	case DistanceEuclideanSquared:
		return EuclideanSquared
	case DistanceCosine:
		fallthrough
	default:
		return Cosine
	}
}

// IsHigherBetter returns true if higher scores indicate better matches.
func IsHigherBetter(t DistanceType) bool {
	switch t {
	case DistanceEuclidean, DistanceEuclideanSquared:
		return false
	default:
		return true
	}
}
