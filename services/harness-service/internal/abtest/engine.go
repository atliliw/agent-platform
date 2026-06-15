// Package abtest provides A/B testing functionality with statistical analysis
package abtest

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExperimentStatus represents the status of an experiment
type ExperimentStatus string

const (
	StatusRunning ExperimentStatus = "running"
	StatusPaused  ExperimentStatus = "paused"
	StatusStopped ExperimentStatus = "stopped"
)

// Experiment represents an A/B test experiment
type Experiment struct {
	ID            string           `gorm:"primaryKey"`
	Name          string           `gorm:"index"`
	AgentID       string           `gorm:"index"`
	ControlConfig string           // JSON config for control
	VariantConfig string           // JSON config for variant
	TrafficSplit  float64          // 0.0 - 1.0, percentage for variant
	Status        ExperimentStatus `gorm:"index"`
	AutoPromote   bool             // Auto-promote when significant
	AutoPromoteThreshold float64   // P-value threshold for auto-promote
	TenantID      string           `gorm:"index"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	StartedAt     *time.Time
	EndedAt       *time.Time
}

// ExperimentResult represents a recorded result
type ExperimentResult struct {
	ID           string    `gorm:"primaryKey"`
	ExperimentID string    `gorm:"index"`
	IsVariant    bool      `gorm:"index"`
	Score        float64   // Quality score
	LatencyMs    float64   // Latency in milliseconds
	Success      bool      // Whether the request succeeded
	SessionID    string    `gorm:"index"` // For tracking unique users
	CreatedAt    time.Time `gorm:"index"`
}

// ExperimentStats holds statistical analysis results
type ExperimentStats struct {
	ControlCount      int64
	VariantCount      int64
	ControlMean       float64
	VariantMean       float64
	ControlStdDev     float64
	VariantStdDev     float64
	Delta             float64
	DeltaPercent      float64
	PValue            float64
	TStatistic        float64
	Significant       bool
	ConfidenceLevel   float64
	RecommendedAction string // "promote_variant", "promote_control", "continue"
}

// Engine is the A/B test engine with database persistence
type Engine struct {
	db          *gorm.DB
	experiments map[string]*experimentState
	mu          sync.RWMutex
}

type experimentState struct {
	controlScores  []float64
	variantScores  []float64
	controlTimes   []float64
	variantTimes   []float64
	controlSuccess int64
	variantSuccess int64
	controlFail    int64
	variantFail    int64
	sessionHashes  map[string]bool // Track unique sessions
}

// NewEngine creates a new A/B test engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:          db,
		experiments: make(map[string]*experimentState),
	}
	// Load existing experiments into memory
	e.loadExperiments()
	return e
}

// NewEngineMemory creates an in-memory only engine (for testing)
func NewEngineMemory() *Engine {
	return &Engine{
		experiments: make(map[string]*experimentState),
	}
}

// loadExperiments loads existing experiments from database
func (e *Engine) loadExperiments() {
	if e.db == nil {
		return
	}

	var experiments []Experiment
	if err := e.db.Find(&experiments).Error; err != nil {
		return
	}

	for _, exp := range experiments {
		if exp.Status == StatusRunning {
			e.experiments[exp.ID] = &experimentState{
				sessionHashes: make(map[string]bool),
			}
		}
	}
}

// CreateExperiment creates a new A/B test experiment
func (e *Engine) CreateExperiment(ctx context.Context, exp *Experiment) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if exp.ID == "" {
		exp.ID = uuid.New().String()
	}
	exp.CreatedAt = time.Now()
	exp.UpdatedAt = time.Now()
	if exp.Status == "" {
		exp.Status = StatusRunning
	}
	if exp.TrafficSplit <= 0 || exp.TrafficSplit > 1 {
		exp.TrafficSplit = 0.5
	}

	// Pause other running experiments for the same agent
	if e.db != nil {
		e.db.Model(&Experiment{}).
			Where("agent_id = ? AND status = ?", exp.AgentID, StatusRunning).
			Update("status", StatusPaused)
	}

	if e.db != nil {
		if err := e.db.Create(exp).Error; err != nil {
			return fmt.Errorf("create experiment: %w", err)
		}
	}

	e.experiments[exp.ID] = &experimentState{
		sessionHashes: make(map[string]bool),
	}

	return nil
}

// GetExperiment retrieves an experiment by ID
func (e *Engine) GetExperiment(ctx context.Context, id string) (*Experiment, error) {
	if e.db != nil {
		var exp Experiment
		if err := e.db.First(&exp, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get experiment: %w", err)
		}
		return &exp, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	return nil, fmt.Errorf("experiment not found")
}

// ListExperiments lists experiments with optional filters
func (e *Engine) ListExperiments(ctx context.Context, agentID, tenantID string, status ExperimentStatus) ([]*Experiment, error) {
	if e.db == nil {
		return nil, nil
	}

	query := e.db.Model(&Experiment{})
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var experiments []*Experiment
	if err := query.Order("created_at DESC").Find(&experiments).Error; err != nil {
		return nil, fmt.Errorf("list experiments: %w", err)
	}

	return experiments, nil
}

// Pause pauses an experiment
func (e *Engine) Pause(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Model(&Experiment{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":     StatusPaused,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("pause experiment: %w", err)
		}
	}

	delete(e.experiments, id)
	return nil
}

// Resume resumes a paused experiment
func (e *Engine) Resume(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		var exp Experiment
		if err := e.db.First(&exp, "id = ?", id).Error; err != nil {
			return fmt.Errorf("get experiment: %w", err)
		}

		// Pause other running experiments for the same agent
		e.db.Model(&Experiment{}).
			Where("agent_id = ? AND status = ? AND id != ?", exp.AgentID, StatusRunning, id).
			Update("status", StatusPaused)

		if err := e.db.Model(&exp).Updates(map[string]interface{}{
			"status":     StatusRunning,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("resume experiment: %w", err)
		}
	}

	e.experiments[id] = &experimentState{
		sessionHashes: make(map[string]bool),
	}
	return nil
}

// Promote promotes the variant as the winner
func (e *Engine) Promote(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	if e.db != nil {
		if err := e.db.Model(&Experiment{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":     StatusStopped,
				"ended_at":   &now,
				"updated_at": now,
			}).Error; err != nil {
			return fmt.Errorf("promote experiment: %w", err)
		}
	}

	delete(e.experiments, id)
	return nil
}

// Reject rejects the variant, keeping control
func (e *Engine) Reject(ctx context.Context, id string) error {
	return e.Promote(ctx, id)
}

// Delete deletes an experiment and its results
func (e *Engine) Delete(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		// Delete results first
		e.db.Where("experiment_id = ?", id).Delete(&ExperimentResult{})
		// Delete experiment
		if err := e.db.Delete(&Experiment{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete experiment: %w", err)
		}
	}

	delete(e.experiments, id)
	return nil
}

// ShouldUseVariant determines if a request should use variant using hash-based assignment
// This ensures consistent assignment for the same session/user
func (e *Engine) ShouldUseVariant(ctx context.Context, experimentID, sessionID string) (bool, error) {
	e.mu.RLock()
	_, exists := e.experiments[experimentID]
	e.mu.RUnlock()

	if !exists {
		// Check database for experiment
		if e.db != nil {
			var exp Experiment
			if err := e.db.First(&exp, "id = ? AND status = ?", experimentID, StatusRunning).Error; err != nil {
				return false, fmt.Errorf("experiment not found or not running")
			}
			// Use traffic split from database
			return hashBasedAssignment(experimentID, sessionID, exp.TrafficSplit), nil
		}
		return false, fmt.Errorf("experiment not found")
	}

	// Get traffic split from database
	trafficSplit := 0.5
	if e.db != nil {
		var exp Experiment
		if err := e.db.Select("traffic_split").First(&exp, "id = ?", experimentID).Error; err == nil {
			trafficSplit = exp.TrafficSplit
		}
	}

	return hashBasedAssignment(experimentID, sessionID, trafficSplit), nil
}

// hashBasedAssignment uses MD5 hash for consistent assignment
func hashBasedAssignment(experimentID, sessionID string, trafficSplit float64) bool {
	// Create hash from experimentID + sessionID
	h := md5.New()
	h.Write([]byte(experimentID + ":" + sessionID))
	hashBytes := h.Sum(nil)

	// Convert first 8 bytes to uint64
	hashValue := binary.BigEndian.Uint64(hashBytes[:8])

	// Normalize to 0-1 range
	normalized := float64(hashValue) / float64(1<<64)

	return normalized < trafficSplit
}

// RecordResult records a result for an experiment
func (e *Engine) RecordResult(ctx context.Context, experimentID, sessionID string, isVariant bool, score, latencyMs float64, success bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, exists := e.experiments[experimentID]
	if !exists {
		state = &experimentState{
			sessionHashes: make(map[string]bool),
		}
		e.experiments[experimentID] = state
	}

	// Record metrics
	if isVariant {
		state.variantScores = append(state.variantScores, score)
		state.variantTimes = append(state.variantTimes, latencyMs)
		if success {
			state.variantSuccess++
		} else {
			state.variantFail++
		}
	} else {
		state.controlScores = append(state.controlScores, score)
		state.controlTimes = append(state.controlTimes, latencyMs)
		if success {
			state.controlSuccess++
		} else {
			state.controlFail++
		}
	}

	// Track unique sessions
	state.sessionHashes[sessionID] = true

	// Persist to database
	if e.db != nil {
		result := &ExperimentResult{
			ID:           uuid.New().String(),
			ExperimentID: experimentID,
			IsVariant:    isVariant,
			Score:        score,
			LatencyMs:    latencyMs,
			Success:      success,
			SessionID:    sessionID,
			CreatedAt:    time.Now(),
		}
		if err := e.db.Create(result).Error; err != nil {
			return fmt.Errorf("record result: %w", err)
		}
	}

	return nil
}

// Evaluate performs statistical analysis on experiment results
func (e *Engine) Evaluate(ctx context.Context, experimentID string) (*ExperimentStats, error) {
	// Load results from database
	var results []ExperimentResult
	if e.db != nil {
		if err := e.db.Where("experiment_id = ?", experimentID).Find(&results).Error; err != nil {
			return nil, fmt.Errorf("load results: %w", err)
		}
	}

	e.mu.RLock()
	state, exists := e.experiments[experimentID]
	e.mu.RUnlock()

	// Aggregate results
	controlScores := make([]float64, 0)
	variantScores := make([]float64, 0)

	// From database results
	for _, r := range results {
		if r.IsVariant {
			variantScores = append(variantScores, r.Score)
		} else {
			controlScores = append(controlScores, r.Score)
		}
	}

	// From in-memory state
	if exists {
		controlScores = append(controlScores, state.controlScores...)
		variantScores = append(variantScores, state.variantScores...)
	}

	stats := &ExperimentStats{
		ControlCount: int64(len(controlScores)),
		VariantCount: int64(len(variantScores)),
	}

	if stats.ControlCount < 2 || stats.VariantCount < 2 {
		stats.RecommendedAction = "continue"
		return stats, nil
	}

	// Calculate means
	stats.ControlMean = mean(controlScores)
	stats.VariantMean = mean(variantScores)

	// Calculate standard deviations
	stats.ControlStdDev = stdDev(controlScores)
	stats.VariantStdDev = stdDev(variantScores)

	// Calculate delta
	stats.Delta = stats.VariantMean - stats.ControlMean
	if stats.ControlMean != 0 {
		stats.DeltaPercent = (stats.Delta / stats.ControlMean) * 100
	}

	// Perform t-test
	stats.TStatistic, stats.PValue = tTest(controlScores, variantScores)
	stats.Significant = stats.PValue < 0.05
	stats.ConfidenceLevel = 1 - stats.PValue

	// Determine recommendation
	if stats.Significant {
		if stats.Delta > 0 {
			stats.RecommendedAction = "promote_variant"
		} else {
			stats.RecommendedAction = "promote_control"
		}
	} else {
		stats.RecommendedAction = "continue"
	}

	return stats, nil
}

// GetStats returns current experiment statistics
func (e *Engine) GetStats(ctx context.Context, experimentID string) (*ExperimentStats, error) {
	return e.Evaluate(ctx, experimentID)
}

// CheckAutoPromote checks if auto-promotion should trigger
func (e *Engine) CheckAutoPromote(ctx context.Context, experimentID string) (bool, string, error) {
	if e.db == nil {
		return false, "", nil
	}

	var exp Experiment
	if err := e.db.First(&exp, "id = ?", experimentID).Error; err != nil {
		return false, "", err
	}

	if !exp.AutoPromote {
		return false, "", nil
	}

	stats, err := e.Evaluate(ctx, experimentID)
	if err != nil {
		return false, "", err
	}

	// Check if we have enough samples (minimum 30 per group)
	if stats.ControlCount < 30 || stats.VariantCount < 30 {
		return false, "insufficient_samples", nil
	}

	// Check if p-value meets threshold
	if stats.PValue < exp.AutoPromoteThreshold && stats.Significant {
		action := "promote_variant"
		if stats.Delta < 0 {
			action = "promote_control"
		}
		// Execute promotion
		if err := e.Promote(ctx, experimentID); err != nil {
			return false, "", err
		}
		return true, action, nil
	}

	return false, "not_significant", nil
}

// Calculate required sample size for desired power
func (e *Engine) CalculateSampleSize(effectSize float64, alpha, power float64) int64 {
	// Simplified sample size calculation
	// n = 2 * (Z_alpha/2 + Z_beta)^2 * sigma^2 / effect_size^2

	zAlpha := 1.96  // 95% confidence
	zBeta := 0.842  // 80% power (approximation)

	n := 2 * math.Pow(zAlpha+zBeta, 2) / math.Pow(effectSize, 2)
	return int64(math.Ceil(n))
}

// Helper functions

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func variance(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	var sum float64
	for _, v := range values {
		diff := v - m
		sum += diff * diff
	}
	return sum / float64(len(values)-1) // Sample variance
}

func stdDev(values []float64) float64 {
	return math.Sqrt(variance(values))
}

// tTest performs a two-sample t-test
func tTest(group1, group2 []float64) (float64, float64) {
	n1 := float64(len(group1))
	n2 := float64(len(group2))

	if n1 < 2 || n2 < 2 {
		return 0, 1.0
	}

	mean1 := mean(group1)
	mean2 := mean(group2)
	var1 := variance(group1)
	var2 := variance(group2)

	// Welch's t-test (unequal variances)
	se := math.Sqrt(var1/n1 + var2/n2)
	if se == 0 {
		return 0, 1.0
	}

	t := (mean1 - mean2) / se

	// Degrees of freedom (Welch-Satterthwaite equation)
	dfNum := math.Pow(var1/n1+var2/n2, 2)
	dfDenom := math.Pow(var1/n1, 2)/(n1-1) + math.Pow(var2/n2, 2)/(n2-1)
	df := dfNum / dfDenom

	// P-value (two-tailed)
	p := 2 * (1 - tDistributionCDF(math.Abs(t), df))

	return t, p
}

// tDistributionCDF approximates the CDF of t-distribution
func tDistributionCDF(t, df float64) float64 {
	// Use normal approximation for large df
	if df > 100 {
		return normalCDF(t)
	}

	// Approximation for small df
	x := df / (df + t*t)
	return 1 - 0.5*betaIncomplete(df/2, 0.5, x)
}

// normalCDF is the standard normal CDF
func normalCDF(x float64) float64 {
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

// betaIncomplete approximates the incomplete beta function
func betaIncomplete(a, b, x float64) float64 {
	if x < 0 || x > 1 {
		return 0
	}

	// Use continued fraction approximation
	if x == 0 {
		return 0
	}
	if x == 1 {
		return 1
	}

	// Simple approximation for moderate values
	lb := math.Log(x)*a + math.Log(1-x)*b
	return math.Exp(lb) * (1 + (a+b)/(a+1) * (1-x))
}
