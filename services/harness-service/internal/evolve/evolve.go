// Package evolve provides self-evolution capabilities including optimizer and proposals
package evolve

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProposalStatus represents the status of a proposal
type ProposalStatus string

const (
	ProposalPending   ProposalStatus = "pending"
	ProposalApproved  ProposalStatus = "approved"
	ProposalRejected  ProposalStatus = "rejected"
	ProposalRunning   ProposalStatus = "running"
	ProposalCompleted ProposalStatus = "completed"
	ProposalFailed    ProposalStatus = "failed"
)

// ProposalType represents the type of proposal
type ProposalType string

const (
	ProposalTypeModelSwitch   ProposalType = "model_switch"
	ProposalTypeConfigOptimize ProposalType = "config_optimize"
	ProposalTypeCostReduce    ProposalType = "cost_reduce"
	ProposalTypePerformance   ProposalType = "performance"
	ProposalTypeABTest        ProposalType = "ab_test"
)

// Proposal represents a self-evolution proposal
type Proposal struct {
	ID           string         `gorm:"primaryKey"`
	AgentID      string         `gorm:"index"`
	Type         ProposalType   `gorm:"index"`
	Title        string
	Description  string
	CurrentState string         // JSON
	ProposedState string        // JSON
	ExpectedBenefit float64     // Estimated benefit score
	RiskLevel    string         // "low", "medium", "high"
	Status       ProposalStatus `gorm:"index"`
	ApprovedBy   string
	ApprovedAt   *time.Time
	ExecutedAt   *time.Time
	Result       string         // JSON execution result
	Metadata     string         // JSON
	TenantID     string         `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// OptimizationResult represents an optimization result
type OptimizationResult struct {
	AgentID       string
	Type          string
	CurrentValue  float64
	OptimizedValue float64
	Improvement   float64
	Config        map[string]interface{}
	Confidence    float64
}

// OptimizerConfig represents optimizer configuration
type OptimizerConfig struct {
	MetricWeights map[string]float64 // Metric -> weight
	Constraints   map[string]float64 // Metric -> constraint value
	LearningRate  float64
	Iterations    int
}

// Engine is the self-evolution engine
type Engine struct {
	db        *gorm.DB
	proposals map[string]*Proposal
	optimizer *Optimizer
	mu        sync.RWMutex
}

// Optimizer is the optimization engine
type Optimizer struct {
	config     *OptimizerConfig
	history    map[string][]float64 // parameter -> values
	mu         sync.RWMutex
}

// NewEngine creates a new evolve engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:        db,
		proposals: make(map[string]*Proposal),
		optimizer: NewOptimizer(&OptimizerConfig{
			MetricWeights: map[string]float64{
				"success_rate": 0.4,
				"latency":      0.3,
				"cost":         0.2,
				"quality":      0.1,
			},
			LearningRate: 0.1,
			Iterations:   100,
		}),
	}
	e.loadProposals()
	return e
}

// NewEngineMemory creates an in-memory evolve engine
func NewEngineMemory() *Engine {
	return &Engine{
		proposals: make(map[string]*Proposal),
		optimizer: NewOptimizer(&OptimizerConfig{
			MetricWeights: map[string]float64{
				"success_rate": 0.4,
				"latency":      0.3,
				"cost":         0.2,
				"quality":      0.1,
			},
			LearningRate: 0.1,
			Iterations:   100,
		}),
	}
}

// loadProposals loads proposals from database
func (e *Engine) loadProposals() {
	if e.db == nil {
		return
	}

	var proposals []Proposal
	if err := e.db.Find(&proposals).Error; err != nil {
		return
	}

	for _, p := range proposals {
		e.proposals[p.ID] = &p
	}
}

// ================== Proposal Management ==================

// CreateProposal creates a new proposal
func (e *Engine) CreateProposal(ctx context.Context, proposal *Proposal) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if proposal.ID == "" {
		proposal.ID = uuid.New().String()
	}
	proposal.CreatedAt = time.Now()
	proposal.UpdatedAt = time.Now()
	proposal.Status = ProposalPending

	if e.db != nil {
		if err := e.db.Create(proposal).Error; err != nil {
			return fmt.Errorf("create proposal: %w", err)
		}
	}

	e.proposals[proposal.ID] = proposal
	return nil
}

// GetProposal retrieves a proposal
func (e *Engine) GetProposal(ctx context.Context, id string) (*Proposal, error) {
	e.mu.RLock()
	proposal, exists := e.proposals[id]
	e.mu.RUnlock()

	if exists {
		return proposal, nil
	}

	if e.db != nil {
		var p Proposal
		if err := e.db.First(&p, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get proposal: %w", err)
		}
		return &p, nil
	}

	return nil, fmt.Errorf("proposal not found")
}

// ListProposals lists proposals
func (e *Engine) ListProposals(ctx context.Context, agentID string, status ProposalStatus) ([]*Proposal, error) {
	if e.db != nil {
		query := e.db.Model(&Proposal{})
		if agentID != "" {
			query = query.Where("agent_id = ?", agentID)
		}
		if status != "" {
			query = query.Where("status = ?", status)
		}

		var proposals []*Proposal
		if err := query.Order("created_at DESC").Find(&proposals).Error; err != nil {
			return nil, fmt.Errorf("list proposals: %w", err)
		}
		return proposals, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Proposal
	for _, p := range e.proposals {
		if agentID != "" && p.AgentID != agentID {
			continue
		}
		if status != "" && p.Status != status {
			continue
		}
		result = append(result, p)
	}

	// Sort by created_at
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

// ApproveProposal approves a proposal
func (e *Engine) ApproveProposal(ctx context.Context, id, approvedBy string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	proposal, exists := e.proposals[id]
	if !exists {
		return fmt.Errorf("proposal not found")
	}

	if proposal.Status != ProposalPending {
		return fmt.Errorf("proposal is not pending")
	}

	now := time.Now()
	proposal.Status = ProposalApproved
	proposal.ApprovedBy = approvedBy
	proposal.ApprovedAt = &now
	proposal.UpdatedAt = now

	if e.db != nil {
		e.db.Model(proposal).Updates(map[string]interface{}{
			"status":      ProposalApproved,
			"approved_by": approvedBy,
			"approved_at": now,
			"updated_at":  now,
		})
	}

	return nil
}

// RejectProposal rejects a proposal
func (e *Engine) RejectProposal(ctx context.Context, id, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	proposal, exists := e.proposals[id]
	if !exists {
		return fmt.Errorf("proposal not found")
	}

	if proposal.Status != ProposalPending {
		return fmt.Errorf("proposal is not pending")
	}

	proposal.Status = ProposalRejected
	proposal.UpdatedAt = time.Now()

	if reason != "" {
		var metadata map[string]interface{}
		if proposal.Metadata != "" {
			json.Unmarshal([]byte(proposal.Metadata), &metadata)
		}
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		metadata["rejection_reason"] = reason
		data, _ := json.Marshal(metadata)
		proposal.Metadata = string(data)
	}

	if e.db != nil {
		e.db.Model(proposal).Updates(map[string]interface{}{
			"status":     ProposalRejected,
			"updated_at": time.Now(),
			"metadata":   proposal.Metadata,
		})
	}

	return nil
}

// ExecuteProposal executes an approved proposal
func (e *Engine) ExecuteProposal(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	proposal, exists := e.proposals[id]
	if !exists {
		return fmt.Errorf("proposal not found")
	}

	if proposal.Status != ProposalApproved {
		return fmt.Errorf("proposal is not approved")
	}

	proposal.Status = ProposalRunning
	now := time.Now()
	proposal.ExecutedAt = &now

	if e.db != nil {
		e.db.Model(proposal).Updates(map[string]interface{}{
			"status":     ProposalRunning,
			"executed_at": now,
		})
	}

	// Execute based on type
	var result map[string]interface{}
	var err error

	switch proposal.Type {
	case ProposalTypeModelSwitch:
		result, err = e.executeModelSwitch(proposal)
	case ProposalTypeConfigOptimize:
		result, err = e.executeConfigOptimize(proposal)
	case ProposalTypeCostReduce:
		result, err = e.executeCostReduce(proposal)
	default:
		err = fmt.Errorf("unknown proposal type: %s", proposal.Type)
	}

	if err != nil {
		proposal.Status = ProposalFailed
		result = map[string]interface{}{"error": err.Error()}
	} else {
		proposal.Status = ProposalCompleted
	}

	resultData, _ := json.Marshal(result)
	proposal.Result = string(resultData)
	proposal.UpdatedAt = time.Now()

	if e.db != nil {
		e.db.Model(proposal).Updates(map[string]interface{}{
			"status":     proposal.Status,
			"result":     proposal.Result,
			"updated_at": time.Now(),
		})
	}

	return err
}

// ================== Optimizer ==================

// NewOptimizer creates a new optimizer
func NewOptimizer(config *OptimizerConfig) *Optimizer {
	return &Optimizer{
		config:  config,
		history: make(map[string][]float64),
	}
}

// RunOptimizer runs the optimization algorithm
func (e *Engine) RunOptimizer(ctx context.Context, agentID string, metrics map[string]float64) (*OptimizationResult, error) {
	e.mu.RLock()
	optimizer := e.optimizer
	e.mu.RUnlock()

	return optimizer.Optimize(agentID, metrics)
}

// Optimize performs parameter optimization
func (o *Optimizer) Optimize(agentID string, currentMetrics map[string]float64) (*OptimizationResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Calculate weighted score
	currentScore := o.calculateScore(currentMetrics)

	// Find optimal parameters using gradient-free optimization
	bestConfig := make(map[string]interface{})
	bestScore := currentScore

	// Simple grid search for demonstration
	paramRanges := map[string][]float64{
		"temperature":     {0.0, 0.3, 0.5, 0.7, 1.0},
		"max_tokens":      {500, 1000, 2000, 4000},
		"frequency_penalty": {0.0, 0.5, 1.0, 1.5},
		"presence_penalty":  {0.0, 0.5, 1.0, 1.5},
	}

	// Try different parameter combinations
	for temp := range paramRanges["temperature"] {
		for maxTokens := range paramRanges["max_tokens"] {
			// Simulate score with different parameters
			estimatedScore := o.estimateScore(currentMetrics, map[string]interface{}{
				"temperature": paramRanges["temperature"][temp],
				"max_tokens":  paramRanges["max_tokens"][maxTokens],
			})

			if estimatedScore > bestScore {
				bestScore = estimatedScore
				bestConfig = map[string]interface{}{
					"temperature": paramRanges["temperature"][temp],
					"max_tokens":  paramRanges["max_tokens"][maxTokens],
				}
			}
		}
	}

	improvement := 0.0
	if currentScore > 0 {
		improvement = (bestScore - currentScore) / currentScore * 100
	}

	return &OptimizationResult{
		AgentID:        agentID,
		Type:           "parameter_optimization",
		CurrentValue:   currentScore,
		OptimizedValue: bestScore,
		Improvement:    improvement,
		Config:         bestConfig,
		Confidence:     o.calculateConfidence(currentMetrics),
	}, nil
}

// calculateScore calculates weighted score from metrics
func (o *Optimizer) calculateScore(metrics map[string]float64) float64 {
	var score float64
	for metric, value := range metrics {
		weight := o.config.MetricWeights[metric]
		if weight == 0 {
			weight = 0.1 // Default weight
		}

		// Normalize value
		normalized := value
		switch metric {
		case "success_rate", "quality":
			// Higher is better (0-1)
			normalized = value
		case "latency", "cost":
			// Lower is better - invert
			if value > 0 {
				normalized = 1 / (1 + value/1000)
			}
		}

		score += normalized * weight
	}
	return score
}

// estimateScore estimates score with different parameters
func (o *Optimizer) estimateScore(currentMetrics map[string]float64, params map[string]interface{}) float64 {
	// Simple heuristic estimation
	score := o.calculateScore(currentMetrics)

	// Temperature affects creativity/quality tradeoff
	if temp, ok := params["temperature"].(float64); ok {
		if temp < 0.3 {
			score *= 1.05 // Lower temp = more consistent
		} else if temp > 0.7 {
			score *= 0.95 // Higher temp = more variance
		}
	}

	// Max tokens affects cost
	if maxTokens, ok := params["max_tokens"].(int); ok {
		if maxTokens < 1000 {
			score *= 1.1 // Lower cost
		} else if maxTokens > 3000 {
			score *= 0.95 // Higher cost
		}
	}

	return score
}

// calculateConfidence calculates confidence in optimization
func (o *Optimizer) calculateConfidence(metrics map[string]float64) float64 {
	// More metrics = higher confidence
	confidence := float64(len(metrics)) * 0.15

	// Check metric quality
	for _, value := range metrics {
		if value > 0 {
			confidence += 0.05
		}
	}

	return math.Min(1.0, confidence)
}

// ================== Auto-tuning ==================

// AutoTune performs automatic parameter tuning
func (e *Engine) AutoTune(ctx context.Context, agentID string, metrics map[string]float64) (*Proposal, error) {
	// Run optimizer
	result, err := e.RunOptimizer(ctx, agentID, metrics)
	if err != nil {
		return nil, err
	}

	// Create proposal if improvement is significant
	if result.Improvement > 5 { // 5% improvement threshold
		proposal := &Proposal{
			AgentID:         agentID,
			Type:           ProposalTypeConfigOptimize,
			Title:          fmt.Sprintf("Auto-optimization for %s", agentID),
			Description:    fmt.Sprintf("Potential improvement: %.1f%%", result.Improvement),
			ExpectedBenefit: result.Improvement,
			RiskLevel:      "low",
		}

		currentState, _ := json.Marshal(metrics)
		proposal.CurrentState = string(currentState)

		proposedState, _ := json.Marshal(result.Config)
		proposal.ProposedState = string(proposedState)

		if err := e.CreateProposal(ctx, proposal); err != nil {
			return nil, err
		}

		return proposal, nil
	}

	return nil, nil
}

// ================== Execution Helpers ==================

func (e *Engine) executeModelSwitch(proposal *Proposal) (map[string]interface{}, error) {
	var proposedState map[string]interface{}
	if err := json.Unmarshal([]byte(proposal.ProposedState), &proposedState); err != nil {
		return nil, fmt.Errorf("parse proposed state: %w", err)
	}

	// In a real implementation, this would switch the model
	return map[string]interface{}{
		"success": true,
		"new_model": proposedState["model"],
		"switched_at": time.Now(),
	}, nil
}

func (e *Engine) executeConfigOptimize(proposal *Proposal) (map[string]interface{}, error) {
	var proposedState map[string]interface{}
	if err := json.Unmarshal([]byte(proposal.ProposedState), &proposedState); err != nil {
		return nil, fmt.Errorf("parse proposed state: %w", err)
	}

	// In a real implementation, this would update the configuration
	return map[string]interface{}{
		"success": true,
		"new_config": proposedState,
		"applied_at": time.Now(),
	}, nil
}

func (e *Engine) executeCostReduce(proposal *Proposal) (map[string]interface{}, error) {
	var proposedState map[string]interface{}
	if err := json.Unmarshal([]byte(proposal.ProposedState), &proposedState); err != nil {
		return nil, fmt.Errorf("parse proposed state: %w", err)
	}

	// In a real implementation, this would apply cost reduction
	return map[string]interface{}{
		"success": true,
		"changes": proposedState,
		"savings": proposal.ExpectedBenefit,
	}, nil
}

// DeleteProposal deletes a proposal
func (e *Engine) DeleteProposal(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Delete(&Proposal{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete proposal: %w", err)
		}
	}

	delete(e.proposals, id)
	return nil
}

// SetOptimizerConfig sets the optimizer configuration
func (e *Engine) SetOptimizerConfig(config *OptimizerConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.optimizer = NewOptimizer(config)
}

