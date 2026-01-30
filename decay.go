package veclite

import (
	"math"
	"time"
)

// DecayType specifies the type of temporal decay function to apply.
type DecayType string

const (
	// DecayNone applies no temporal decay.
	DecayNone DecayType = "none"
	// DecayExponential applies exponential decay: score * 2^(-age/halfLife)
	DecayExponential DecayType = "exponential"
	// DecayLinear applies linear decay: score * max(0, 1 - age/maxAge)
	DecayLinear DecayType = "linear"
	// DecayGaussian applies Gaussian decay: score * exp(-0.5 * (age/sigma)^2)
	DecayGaussian DecayType = "gaussian"
)

// DecayConfig holds the configuration for temporal decay.
type DecayConfig struct {
	// Type is the decay function type.
	Type DecayType
	// HalfLife is the time after which the score is halved (for exponential decay).
	// For linear decay, this is the maximum age after which the score is zero.
	// For gaussian decay, this is the sigma parameter.
	HalfLife time.Duration
}

// WithDecay sets the temporal decay function for search results.
// The decay is applied based on record age (time since creation).
func WithDecay(decayType DecayType, halfLife time.Duration) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		if c.decay == nil {
			c.decay = &DecayConfig{}
		}
		c.decay.Type = decayType
		c.decay.HalfLife = halfLife
	})
}

// WithImportanceBoost sets a boost factor for importance scores.
// The final score is: baseScore * (1 + importanceBoost * importance)
// A factor of 1.0 means importance can double the score.
func WithImportanceBoost(factor float32) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		c.importanceBoost = factor
	})
}

// WithAccessTracking enables access tracking for search results.
// When enabled, accessing a record via search increments its access count
// and updates its last accessed timestamp.
func WithAccessTracking(enabled bool) SearchOption {
	return searchOptionFunc(func(c *searchConfig) {
		c.accessTracking = enabled
	})
}

// applyDecay applies the decay function to a score based on record age.
func applyDecay(score float32, age time.Duration, config *DecayConfig) float32 {
	if config == nil || config.Type == DecayNone || config.HalfLife <= 0 {
		return score
	}

	ageSeconds := float64(age) / float64(time.Second)
	halfLifeSeconds := float64(config.HalfLife) / float64(time.Second)

	switch config.Type {
	case DecayExponential:
		// score * 2^(-age/halfLife)
		decayFactor := math.Pow(2, -ageSeconds/halfLifeSeconds)
		return score * float32(decayFactor)

	case DecayLinear:
		// score * max(0, 1 - age/maxAge)
		// HalfLife is used as maxAge for linear decay
		decayFactor := 1 - ageSeconds/halfLifeSeconds
		if decayFactor < 0 {
			decayFactor = 0
		}
		return score * float32(decayFactor)

	case DecayGaussian:
		// score * exp(-0.5 * (age/sigma)^2)
		// HalfLife is used as sigma for gaussian decay
		ratio := ageSeconds / halfLifeSeconds
		decayFactor := math.Exp(-0.5 * ratio * ratio)
		return score * float32(decayFactor)

	default:
		return score
	}
}

// applyImportanceBoost applies importance boost to a score.
func applyImportanceBoost(score float32, importance float32, boost float32) float32 {
	if boost <= 0 {
		return score
	}
	// score * (1 + boost * importance)
	return score * (1 + boost*importance)
}

// applyScoreModifiers applies all score modifiers (decay and importance boost).
func applyScoreModifiers(score float32, record *Record, config *searchConfig) float32 {
	result := score

	// Apply temporal decay
	if config.decay != nil && config.decay.Type != DecayNone {
		age := time.Since(record.CreatedAt)
		result = applyDecay(result, age, config.decay)
	}

	// Apply importance boost
	if config.importanceBoost > 0 {
		result = applyImportanceBoost(result, record.Importance, config.importanceBoost)
	}

	return result
}
