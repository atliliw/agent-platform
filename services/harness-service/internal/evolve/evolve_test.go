package evolve

import (
	"math"
	"testing"
)

// TestEstimateScoreHandlesFloat64MaxTokens locks the fix where estimateScore did
// `params["max_tokens"].(int)` (single-return type assertion), which panics when
// the value is a float64. JSON numbers decode to float64, so any config coming
// through JSON would crash the optimizer. The fix adds a float64/int type switch.
func TestEstimateScoreHandlesFloat64MaxTokens(t *testing.T) {
	// Arrange
	opt := NewOptimizer(&OptimizerConfig{
		MetricWeights: map[string]float64{"success_rate": 1.0},
	})
	metrics := map[string]float64{"success_rate": 1.0} // base score = 1.0

	tests := []struct {
		name      string
		maxTokens interface{}
		wantScore float64
	}{
		// float64 inputs (as produced by encoding/json) must NOT panic:
		{"float64 under 1000 -> x1.1", float64(500), 1.1},
		{"float64 over 3000 -> x0.95", float64(4000), 0.95},
		{"float64 mid range -> unchanged", float64(2000), 1.0},
		// int inputs still work:
		{"int under 1000 -> x1.1", 500, 1.1},
		{"int over 3000 -> x0.95", 4000, 0.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			// A panic here means the type-assertion regression returned.
			got := opt.estimateScore(metrics, map[string]interface{}{
				"max_tokens": tt.maxTokens,
			})

			// Assert
			if math.Abs(got-tt.wantScore) > 1e-9 {
				t.Errorf("estimateScore(max_tokens=%v) = %v, want %v", tt.maxTokens, got, tt.wantScore)
			}
		})
	}
}

// TestEstimateScoreTemperatureStillApplied ensures the temperature adjustment
// was not broken by the max_tokens type-switch fix.
func TestEstimateScoreTemperatureStillApplied(t *testing.T) {
	// Arrange
	opt := NewOptimizer(&OptimizerConfig{
		MetricWeights: map[string]float64{"success_rate": 1.0},
	})
	metrics := map[string]float64{"success_rate": 1.0}

	// Act
	lowTemp := opt.estimateScore(metrics, map[string]interface{}{"temperature": float64(0.1)})
	highTemp := opt.estimateScore(metrics, map[string]interface{}{"temperature": float64(0.9)})

	// Assert: low temp multiplies by 1.05, high temp by 0.95
	if math.Abs(lowTemp-1.05) > 1e-9 {
		t.Errorf("low temp score = %v, want 1.05", lowTemp)
	}
	if math.Abs(highTemp-0.95) > 1e-9 {
		t.Errorf("high temp score = %v, want 0.95", highTemp)
	}
}
