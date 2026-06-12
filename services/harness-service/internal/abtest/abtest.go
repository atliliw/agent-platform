// Package abtest provides A/B testing functionality
package abtest

import (
	"math"
	"sync"

	pb "agent-platform/pkg/pb/harness"
)

// Engine is the A/B test engine
type Engine struct {
	tests  map[string]*testState
	trials map[string]*trialData
	mu     sync.RWMutex
}

type testState struct {
	controlScores []float64
	variantScores []float64
	controlTimes  []float64
	variantTimes  []float64
}

type trialData struct {
	controlCount int
	variantCount int
	splitRatio   float64
}

// NewEngine creates a new A/B test engine
func NewEngine() *Engine {
	return &Engine{
		tests:  make(map[string]*testState),
		trials: make(map[string]*trialData),
	}
}

// RecordScore records a score for a variant
func (e *Engine) RecordScore(testID string, isVariant bool, score float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, exists := e.tests[testID]
	if !exists {
		state = &testState{}
		e.tests[testID] = state
	}

	if isVariant {
		state.variantScores = append(state.variantScores, score)
	} else {
		state.controlScores = append(state.controlScores, score)
	}
}

// RecordLatency records latency for a variant
func (e *Engine) RecordLatency(testID string, isVariant bool, latencyMs float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, exists := e.tests[testID]
	if !exists {
		state = &testState{}
		e.tests[testID] = state
	}

	if isVariant {
		state.variantTimes = append(state.variantTimes, latencyMs)
	} else {
		state.controlTimes = append(state.controlTimes, latencyMs)
	}
}

// AssignVariant assigns a request to control or variant
func (e *Engine) AssignVariant(testID string, splitRatio float64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	trial, exists := e.trials[testID]
	if !exists {
		trial = &trialData{splitRatio: splitRatio}
		e.trials[testID] = trial
	}

	// Assign based on current ratio
	total := float64(trial.controlCount + trial.variantCount)
	if total == 0 {
		// First request, assign to variant based on split ratio
		return false // Start with control
	}

	currentVariantRatio := float64(trial.variantCount) / total
	if currentVariantRatio < splitRatio {
		trial.variantCount++
		return true
	}
	trial.controlCount++
	return false
}

// GetResult gets the A/B test result
func (e *Engine) GetResult(testID string) (*pb.ABTestResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, exists := e.tests[testID]
	if !exists {
		return &pb.ABTestResult{}, nil
	}

	controlAvg := average(state.controlScores)
	variantAvg := average(state.variantScores)

	delta := variantAvg - controlAvg
	nControl := len(state.controlScores)
	nVariant := len(state.variantScores)

	// Statistical significance check (simplified)
	// Using rule of thumb: need at least 30 samples each
	significant := nControl >= 30 && nVariant >= 30

	// Calculate p-value approximation (simplified)
	pValue := calculatePValue(state.controlScores, state.variantScores)

	recommended := "continue"
	if significant {
		if delta > 0.05 {
			recommended = "variant"
		} else if delta < -0.05 {
			recommended = "control"
		}
	}

	return &pb.ABTestResult{
		ControlScore: controlAvg,
		VariantScore: variantAvg,
		Delta:        delta,
		PValue:       pValue,
		Significant:  significant,
		Recommended:  recommended,
	}, nil
}

// average calculates the mean of a slice
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// variance calculates the variance of a slice
func variance(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := average(values)
	var sum float64
	for _, v := range values {
		diff := v - mean
		sum += diff * diff
	}
	return sum / float64(len(values))
}

// calculatePValue calculates a simplified p-value
func calculatePValue(control, variant []float64) float64 {
	n1 := float64(len(control))
	n2 := float64(len(variant))

	if n1 < 2 || n2 < 2 {
		return 1.0
	}

	mean1 := average(control)
	mean2 := average(variant)
	var1 := variance(control)
	var2 := variance(variant)

	// Pooled standard error
	se := math.Sqrt(var1/n1 + var2/n2)
	if se == 0 {
		return 1.0
	}

	// Z-score
	z := (mean2 - mean1) / se

	// Approximate p-value (two-tailed)
	// Using a simple approximation
	p := 2 * (1 - normalCDF(math.Abs(z)))
	return p
}

// normalCDF is the cumulative distribution function for standard normal
func normalCDF(x float64) float64 {
	// Approximation of the normal CDF
	// Abramowitz and Stegun approximation
	const (
		a1 = 0.254829592
		a2 = -0.284496736
		a3 = 1.421413741
		a4 = -1.453152027
		a5 = 1.061405429
		p  = 0.3275911
	)

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = math.Abs(x) / math.Sqrt(2)

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return 0.5 * (1.0 + sign*y)
}
