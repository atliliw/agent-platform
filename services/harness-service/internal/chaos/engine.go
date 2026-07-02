// Package chaos provides chaos engineering functionality
package chaos

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExperimentStatus represents the status of a chaos experiment
type ExperimentStatus string

const (
	StatusCreated   ExperimentStatus = "created"
	StatusRunning   ExperimentStatus = "running"
	StatusPaused    ExperimentStatus = "paused"
	StatusCompleted ExperimentStatus = "completed"
	StatusFailed    ExperimentStatus = "failed"
	StatusStopped   ExperimentStatus = "stopped"
)

// FaultType represents the type of fault to inject
type FaultType string

const (
	FaultAgentTimeout    FaultType = "agent_timeout"
	FaultAgentError      FaultType = "agent_error"
	FaultModelDegraded   FaultType = "model_degraded"
	FaultGuardrailBypass FaultType = "guardrail_bypass"
	FaultBudgetExhaust   FaultType = "budget_exhaust"
	FaultNetworkLatency  FaultType = "network_latency"
	FaultMemoryExhaust   FaultType = "memory_exhaust"
	FaultRateLimit       FaultType = "rate_limit"
	FaultToolFailure     FaultType = "tool_failure"
	FaultContextLoss     FaultType = "context_loss"
)

// Experiment represents a chaos experiment
type Experiment struct {
	ID             string           `gorm:"primaryKey"`
	Name           string
	Description    string
	AgentID        string           `gorm:"index"`
	FaultType      FaultType        `gorm:"index"`
	FaultConfig    string           // JSON configuration for fault
	Duration       int              // Duration in minutes
	BlastRadius    float64          // 0-1, percentage of traffic to affect
	AutoStopOnSLO  bool             // Stop if SLO breach detected
	SLOThreshold   float64          // SLO threshold for auto-stop
	Status         ExperimentStatus `gorm:"index"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	EndedAt        *time.Time
	TenantID       string           `gorm:"index"`
}

// ExperimentRun represents a single run of a chaos experiment
type ExperimentRun struct {
	ID           string           `gorm:"primaryKey"`
	ExperimentID string           `gorm:"index"`
	Status       ExperimentStatus
	StartedAt    time.Time
	EndedAt      *time.Time
	FaultsInjected int64          // Number of faults injected
	RequestsAffected int64        // Number of requests affected
	AutoStopped  bool             // Was auto-stopped by SLO
	SLOBreachAt  *time.Time       // When SLO was breached
	Result       string           // JSON result summary
}

// FaultInjection represents a single fault injection event
type FaultInjection struct {
	ID           string    `gorm:"primaryKey"`
	RunID        string    `gorm:"index"`
	ExperimentID string    `gorm:"index"`
	RequestID    string    `gorm:"index"`
	SessionID    string
	FaultType    FaultType
	InjectedAt   time.Time
	DurationMs   int64
	Success      bool
	Error        string
}

// FaultConfig represents configuration for a specific fault type
type FaultConfig struct {
	TimeoutMs    int64   `json:"timeout_ms,omitempty"`
	ErrorRate    float64 `json:"error_rate,omitempty"`
	ErrorMessage string  `json:"error_message,omitempty"`
	DegradationFactor float64 `json:"degradation_factor,omitempty"`
	LatencyMs    int64   `json:"latency_ms,omitempty"`
	MemoryLimit  int64   `json:"memory_limit,omitempty"`
	BudgetLimit  float64 `json:"budget_limit,omitempty"`
	Message      string  `json:"message,omitempty"`
}

// FaultEffect represents the actual effect to apply
type FaultEffect struct {
	// Delay to add (milliseconds)
	AddLatencyMs int64
	// Whether to force timeout
	ForceTimeout bool
	TimeoutMs    int64
	// Whether to inject error
	ForceError bool
	ErrorMessage string
	// Quality degradation (0-1, multiply response quality)
	QualityFactor float64
	// Budget override
	BudgetOverride float64
}

// Engine is the chaos engineering engine
type Engine struct {
	db            *gorm.DB
	experiments   map[string]*Experiment
	activeRuns    map[string]*ExperimentRun
	sloCheckFunc  func(string) (float64, error) // Function to check SLO status
	mu            sync.RWMutex
	rng           *rand.Rand
}

// NewEngine creates a new chaos engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:          db,
		experiments: make(map[string]*Experiment),
		activeRuns:  make(map[string]*ExperimentRun),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	e.loadExperiments()
	return e
}

// NewEngineMemory creates an in-memory chaos engine
func NewEngineMemory() *Engine {
	return &Engine{
		experiments: make(map[string]*Experiment),
		activeRuns:  make(map[string]*ExperimentRun),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// loadExperiments loads experiments from database
func (e *Engine) loadExperiments() {
	if e.db == nil {
		return
	}

	var experiments []Experiment
	if err := e.db.Find(&experiments).Error; err != nil {
		return
	}

	for _, exp := range experiments {
		e.experiments[exp.ID] = &exp
	}
}

// SetSLOChecker sets the SLO checking function
func (e *Engine) SetSLOChecker(checker func(string) (float64, error)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sloCheckFunc = checker
}

// CreateExperiment creates a chaos experiment
func (e *Engine) CreateExperiment(ctx context.Context, exp *Experiment) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if exp.ID == "" {
		exp.ID = uuid.New().String()
	}
	exp.CreatedAt = time.Now()
	exp.UpdatedAt = time.Now()
	exp.Status = StatusCreated

	if exp.Duration <= 0 {
		exp.Duration = 5 // Default 5 minutes
	}
	if exp.BlastRadius <= 0 || exp.BlastRadius > 1 {
		exp.BlastRadius = 0.1 // Default 10% blast radius
	}

	if e.db != nil {
		if err := e.db.Create(exp).Error; err != nil {
			return fmt.Errorf("create experiment: %w", err)
		}
	}

	e.experiments[exp.ID] = exp
	return nil
}

// GetExperiment retrieves a chaos experiment
func (e *Engine) GetExperiment(ctx context.Context, id string) (*Experiment, error) {
	e.mu.RLock()
	exp, exists := e.experiments[id]
	e.mu.RUnlock()

	if exists {
		return exp, nil
	}

	if e.db != nil {
		var expDb Experiment
		if err := e.db.First(&expDb, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get experiment: %w", err)
		}
		return &expDb, nil
	}

	return nil, fmt.Errorf("experiment not found")
}

// ListExperiments lists chaos experiments
func (e *Engine) ListExperiments(ctx context.Context, agentID string, status ExperimentStatus) ([]*Experiment, error) {
	if e.db != nil {
		query := e.db.Model(&Experiment{})
		if agentID != "" {
			query = query.Where("agent_id = ?", agentID)
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

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Experiment
	for _, exp := range e.experiments {
		if agentID != "" && exp.AgentID != agentID {
			continue
		}
		if status != "" && exp.Status != status {
			continue
		}
		result = append(result, exp)
	}
	return result, nil
}

// StartExperiment starts a chaos experiment
func (e *Engine) StartExperiment(ctx context.Context, id string) (*ExperimentRun, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return nil, fmt.Errorf("experiment not found")
	}

	if exp.Status == StatusRunning {
		return nil, fmt.Errorf("experiment already running")
	}

	// Create run
	run := &ExperimentRun{
		ID:           uuid.New().String(),
		ExperimentID: id,
		Status:       StatusRunning,
		StartedAt:    time.Now(),
	}

	if e.db != nil {
		if err := e.db.Create(run).Error; err != nil {
			return nil, fmt.Errorf("create run: %w", err)
		}
	}

	// Update experiment status
	now := time.Now()
	exp.Status = StatusRunning
	exp.StartedAt = &now
	exp.UpdatedAt = now

	if e.db != nil {
		e.db.Model(exp).Updates(map[string]interface{}{
			"status":     StatusRunning,
			"started_at": now,
			"updated_at": now,
		})
	}

	e.activeRuns[run.ID] = run

	// Start monitoring for SLO breach if enabled
	if exp.AutoStopOnSLO && e.sloCheckFunc != nil {
		go e.monitorSLO(exp, run)
	}

	// Schedule experiment end
	go e.scheduleEnd(exp, run)

	return run, nil
}

// StopExperiment stops a running experiment
func (e *Engine) StopExperiment(ctx context.Context, id string, autoStopped bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return fmt.Errorf("experiment not found")
	}

	if exp.Status != StatusRunning {
		return fmt.Errorf("experiment not running")
	}

	now := time.Now()

	// Find active run
	var activeRun *ExperimentRun
	for _, run := range e.activeRuns {
		if run.ExperimentID == id {
			activeRun = run
			break
		}
	}

	if activeRun != nil {
		activeRun.Status = StatusStopped
		activeRun.EndedAt = &now
		activeRun.AutoStopped = autoStopped

		if e.db != nil {
			e.db.Model(activeRun).Updates(map[string]interface{}{
				"status":        StatusStopped,
				"ended_at":      now,
				"auto_stopped":  autoStopped,
			})
		}

		delete(e.activeRuns, activeRun.ID)
	}

	// Update experiment
	exp.Status = StatusStopped
	exp.EndedAt = &now
	exp.UpdatedAt = now

	if e.db != nil {
		e.db.Model(exp).Updates(map[string]interface{}{
			"status":     StatusStopped,
			"ended_at":   now,
			"updated_at": now,
		})
	}

	return nil
}

// monitorSLO monitors SLO during experiment
func (e *Engine) monitorSLO(exp *Experiment, run *ExperimentRun) {
	checkInterval := 30 * time.Second
	duration := time.Duration(exp.Duration) * time.Minute

	for time.Since(run.StartedAt) < duration {
		time.Sleep(checkInterval)

		if e.sloCheckFunc == nil {
			continue
		}

		sloValue, err := e.sloCheckFunc(exp.AgentID)
		if err != nil {
			continue
		}

		// Check if SLO breached threshold
		if sloValue < exp.SLOThreshold {
			now := time.Now()
			run.SLOBreachAt = &now
			e.StopExperiment(context.Background(), exp.ID, true)
			return
		}
	}
}

// scheduleEnd schedules experiment end after duration
func (e *Engine) scheduleEnd(exp *Experiment, run *ExperimentRun) {
	duration := time.Duration(exp.Duration) * time.Minute
	time.Sleep(duration)

	e.mu.RLock()
	currentStatus := exp.Status
	e.mu.RUnlock()

	if currentStatus == StatusRunning {
		e.StopExperiment(context.Background(), exp.ID, false)
	}
}

// ShouldInjectFault checks if a fault should be injected for a request
// Returns: shouldInject, experimentID, faultEffect
func (e *Engine) ShouldInjectFault(ctx context.Context, agentID string) (bool, string, *FaultEffect) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, exp := range e.experiments {
		if exp.AgentID == agentID && exp.Status == StatusRunning {
			// Check blast radius - only inject fault for percentage of requests
			if e.rng.Float64() <= exp.BlastRadius {
				effect := e.computeFaultEffect(exp)
				return true, exp.ID, effect
			}
		}
	}

	return false, "", nil
}

// computeFaultEffect computes the actual effect to apply based on experiment config
func (e *Engine) computeFaultEffect(exp *Experiment) *FaultEffect {
	effect := &FaultEffect{
		QualityFactor: 1.0, // Default: no degradation
	}

	// Parse fault config
	var config FaultConfig
	if exp.FaultConfig != "" {
		if err := json.Unmarshal([]byte(exp.FaultConfig), &config); err == nil {
			// Successfully parsed config
		}
	}

	// Apply fault type specific effects
	switch exp.FaultType {
	case FaultAgentTimeout:
		timeoutMs := config.TimeoutMs
		if timeoutMs == 0 {
			timeoutMs = 30000 // Default 30s
		}
		effect.ForceTimeout = true
		effect.TimeoutMs = timeoutMs

	case FaultAgentError:
		effect.ForceError = true
		effect.ErrorMessage = config.ErrorMessage
		if effect.ErrorMessage == "" {
			effect.ErrorMessage = config.Message
		}
		if effect.ErrorMessage == "" {
			effect.ErrorMessage = "Chaos injection: simulated agent error"
		}

	case FaultModelDegraded:
		factor := config.DegradationFactor
		if factor == 0 {
			factor = 0.5 // Default 50% degradation
		}
		effect.QualityFactor = factor

	case FaultNetworkLatency:
		latencyMs := config.LatencyMs
		if latencyMs == 0 {
			latencyMs = 2000 // Default 2s
		}
		effect.AddLatencyMs = latencyMs

	case FaultBudgetExhaust:
		limit := config.BudgetLimit
		if limit == 0 {
			limit = 0.01 // Nearly exhausted
		}
		effect.BudgetOverride = limit

	case FaultGuardrailBypass:
		// This is handled at the guardrail level
		// Just mark it for now
		effect.ErrorMessage = "Chaos: guardrail bypass requested"

	case FaultRateLimit:
		effect.ForceError = true
		effect.ErrorMessage = "Chaos: rate limit exceeded (429)"

	case FaultToolFailure:
		effect.ForceError = true
		effect.ErrorMessage = "Chaos: tool execution failed"

	case FaultContextLoss:
		effect.ForceError = true
		effect.ErrorMessage = "Chaos: context window exceeded"

	case FaultMemoryExhaust:
		effect.ForceError = true
		effect.ErrorMessage = "Chaos: memory limit exceeded"
	}

	return effect
}

// RecordFaultInjection records that a fault was actually injected
func (e *Engine) RecordFaultInjection(ctx context.Context, experimentID, requestID, sessionID string, effect *FaultEffect, applied bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Create fault injection record
	fault := &FaultInjection{
		ID:           uuid.New().String(),
		ExperimentID: experimentID,
		RequestID:    requestID,
		SessionID:    sessionID,
		FaultType:    e.experiments[experimentID].FaultType,
		InjectedAt:   time.Now(),
		DurationMs:   effect.AddLatencyMs,
		Success:      applied,
	}

	// Find active run and increment counts
	for _, run := range e.activeRuns {
		if run.ExperimentID == experimentID {
			run.FaultsInjected++
			if applied {
				run.RequestsAffected++
			}
			fault.RunID = run.ID
			break
		}
	}

	if e.db != nil {
		if err := e.db.Create(fault).Error; err != nil {
			return fmt.Errorf("record fault injection: %w", err)
		}

		// Update run counts in DB
		for _, run := range e.activeRuns {
			if run.ExperimentID == experimentID {
				e.db.Model(run).Updates(map[string]interface{}{
					"faults_injected":   run.FaultsInjected,
					"requests_affected": run.RequestsAffected,
				})
				break
			}
		}
	}

	return nil
}

// ApplyLatency actually applies latency (blocking sleep)
func (e *Engine) ApplyLatency(effect *FaultEffect) {
	if effect.AddLatencyMs > 0 {
		time.Sleep(time.Duration(effect.AddLatencyMs) * time.Millisecond)
	}
}

// GetFaultConfig returns the fault configuration for an experiment
func (e *Engine) GetFaultConfig(ctx context.Context, experimentID string) (*FaultConfig, error) {
	exp, err := e.GetExperiment(ctx, experimentID)
	if err != nil {
		return nil, err
	}

	config := &FaultConfig{}
	if exp.FaultConfig != "" {
		if err := json.Unmarshal([]byte(exp.FaultConfig), config); err != nil {
			// Return default config on parse error
		}
	}

	// Set defaults based on fault type
	switch exp.FaultType {
	case FaultAgentTimeout:
		if config.TimeoutMs == 0 {
			config.TimeoutMs = 30000
		}
	case FaultNetworkLatency:
		if config.LatencyMs == 0 {
			config.LatencyMs = 2000
		}
	case FaultAgentError:
		if config.ErrorRate == 0 {
			config.ErrorRate = 0.5
		}
	}

	return config, nil
}

// ListRuns lists experiment runs
func (e *Engine) ListRuns(ctx context.Context, experimentID string, limit int) ([]*ExperimentRun, error) {
	if e.db != nil {
		var runs []*ExperimentRun
		query := e.db.Where("experiment_id = ?", experimentID).Order("started_at DESC")
		if limit > 0 {
			query = query.Limit(limit)
		}
		if err := query.Find(&runs).Error; err != nil {
			return nil, fmt.Errorf("list runs: %w", err)
		}
		return runs, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*ExperimentRun
	for _, run := range e.activeRuns {
		if run.ExperimentID == experimentID {
			result = append(result, run)
		}
	}
	return result, nil
}

// GetRun retrieves a specific run
func (e *Engine) GetRun(ctx context.Context, runID string) (*ExperimentRun, error) {
	e.mu.RLock()
	run, exists := e.activeRuns[runID]
	e.mu.RUnlock()

	if exists {
		return run, nil
	}

	if e.db != nil {
		var r ExperimentRun
		if err := e.db.First(&r, "id = ?", runID).Error; err != nil {
			return nil, fmt.Errorf("get run: %w", err)
		}
		return &r, nil
	}

	return nil, fmt.Errorf("run not found")
}

// DeleteExperiment deletes a chaos experiment
func (e *Engine) DeleteExperiment(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		e.db.Where("experiment_id = ?", id).Delete(&ExperimentRun{})
		e.db.Where("experiment_id = ?", id).Delete(&FaultInjection{})
		if err := e.db.Delete(&Experiment{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete experiment: %w", err)
		}
	}

	delete(e.experiments, id)
	return nil
}

// PauseExperiment pauses a running experiment
func (e *Engine) PauseExperiment(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return fmt.Errorf("experiment not found")
	}

	if exp.Status != StatusRunning {
		return fmt.Errorf("experiment not running")
	}

	exp.Status = StatusPaused
	exp.UpdatedAt = time.Now()

	if e.db != nil {
		e.db.Model(exp).Updates(map[string]interface{}{
			"status":     StatusPaused,
			"updated_at": time.Now(),
		})
	}

	return nil
}

// ResumeExperiment resumes a paused experiment
func (e *Engine) ResumeExperiment(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return fmt.Errorf("experiment not found")
	}

	if exp.Status != StatusPaused {
		return fmt.Errorf("experiment not paused")
	}

	exp.Status = StatusRunning
	exp.UpdatedAt = time.Now()

	if e.db != nil {
		e.db.Model(exp).Updates(map[string]interface{}{
			"status":     StatusRunning,
			"updated_at": time.Now(),
		})
	}

	return nil
}

// GetActiveExperiments returns all active experiments for an agent
func (e *Engine) GetActiveExperiments(ctx context.Context, agentID string) []*Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Experiment
	for _, exp := range e.experiments {
		if exp.AgentID == agentID && exp.Status == StatusRunning {
			result = append(result, exp)
		}
	}
	return result
}

// ClearAllFaults stops all running experiments
func (e *Engine) ClearAllFaults(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, exp := range e.experiments {
		if exp.Status == StatusRunning {
			exp.Status = StatusStopped
			exp.EndedAt = &time.Time{}
			exp.UpdatedAt = time.Now()

			if e.db != nil {
				e.db.Model(exp).Updates(map[string]interface{}{
					"status":     StatusStopped,
					"ended_at":   time.Now(),
					"updated_at": time.Now(),
				})
			}
		}
	}

	e.activeRuns = make(map[string]*ExperimentRun)
	return nil
}

// GetStats returns statistics about chaos experiments
func (e *Engine) GetStats(ctx context.Context) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var total, running, stopped, paused int
	for _, exp := range e.experiments {
		total++
		switch exp.Status {
		case StatusRunning:
			running++
		case StatusStopped:
			stopped++
		case StatusPaused:
			paused++
		}
	}

	var totalFaultsInjected, totalRequestsAffected int64
	for _, run := range e.activeRuns {
		totalFaultsInjected += run.FaultsInjected
		totalRequestsAffected += run.RequestsAffected
	}

	return map[string]interface{}{
		"total_experiments":     total,
		"running_experiments":   running,
		"stopped_experiments":   stopped,
		"paused_experiments":    paused,
		"active_runs":           len(e.activeRuns),
		"total_faults_injected": totalFaultsInjected,
		"total_requests_affected": totalRequestsAffected,
	}
}