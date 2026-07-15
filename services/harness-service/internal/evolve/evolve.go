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

// AgentUpdater is a callback that updates an agent's configuration
type AgentUpdater func(ctx context.Context, agentID string, updates map[string]interface{}) error

// Engine is the self-evolution engine
type Engine struct {
	db           *gorm.DB
	proposals    map[string]*Proposal
	optimizer    *Optimizer
	agentUpdater AgentUpdater
	mu           sync.RWMutex
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

// SetAgentUpdater sets the agent updater callback
func (e *Engine) SetAgentUpdater(updater AgentUpdater) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.agentUpdater = updater
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
	newModel, _ := proposedState["model"].(string)
	if newModel == "" {
		return nil, fmt.Errorf("proposed state missing 'model' field")
	}
	if e.agentUpdater != nil {
		ctx := context.Background()
		if err := e.agentUpdater(ctx, proposal.AgentID, map[string]interface{}{"model": newModel}); err != nil {
			return nil, fmt.Errorf("update agent model: %w", err)
		}
	}
	return map[string]interface{}{
		"success":     true,
		"new_model":   newModel,
		"switched_at": time.Now(),
	}, nil
}

func (e *Engine) executeConfigOptimize(proposal *Proposal) (map[string]interface{}, error) {
	var proposedState map[string]interface{}
	if err := json.Unmarshal([]byte(proposal.ProposedState), &proposedState); err != nil {
		return nil, fmt.Errorf("parse proposed state: %w", err)
	}

	if e.agentUpdater != nil {
		ctx := context.Background()
		if err := e.agentUpdater(ctx, proposal.AgentID, proposedState); err != nil {
			return nil, fmt.Errorf("update agent config: %w", err)
		}
	}

	return map[string]interface{}{
		"success":    true,
		"new_config": proposedState,
		"applied_at": time.Now(),
	}, nil
}

func (e *Engine) executeCostReduce(proposal *Proposal) (map[string]interface{}, error) {
	var proposedState map[string]interface{}
	if err := json.Unmarshal([]byte(proposal.ProposedState), &proposedState); err != nil {
		return nil, fmt.Errorf("parse proposed state: %w", err)
	}
	if e.agentUpdater != nil {
		ctx := context.Background()
		if err := e.agentUpdater(ctx, proposal.AgentID, proposedState); err != nil {
			return nil, fmt.Errorf("apply cost reduction: %w", err)
		}
	}
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

// ================== Auto Analysis ==================

// AnalysisData contains data for proposal analysis
type AnalysisData struct {
	CostData       *CostAnalysisData
	SLOData        *SLOAnalysisData
	ModelData      *ModelAnalysisData
}

// CostAnalysisData contains cost metrics for analysis
type CostAnalysisData struct {
	TotalCost        float64
	ForecastCost     float64
	RequestCount     int64
	InputTokens      int64
	OutputTokens     int64
	AvgCostPerRequest float64
	ByModel          []ModelCostData
}

// ModelCostData contains per-model cost data
type ModelCostData struct {
	ModelID     string
	ModelName   string
	Cost        float64
	RequestCount int64
	InputPrice  float64  // per 1M tokens
	OutputPrice float64  // per 1M tokens
}

// SLOAnalysisData contains SLO metrics for analysis
type SLOAnalysisData struct {
	SLOs []SLOData
}

// SLOData contains single SLO metrics
type SLOData struct {
	ID          string
	Name        string
	Target      float64
	Current     float64
	Status      string
	ErrorBudget float64
	BurnRate    float64
	AgentID     string
}

// ModelAnalysisData contains model comparison data
type ModelAnalysisData struct {
	CurrentModel   string
	CurrentCost    float64
	Alternatives   []AlternativeModel
}

// AlternativeModel represents an alternative model option
type AlternativeModel struct {
	ModelID      string
	ModelName    string
	InputPrice   float64
	OutputPrice  float64
	QualityScore float64  // 0-1, relative quality compared to current
}

// AnalyzeAndPropose analyzes data and generates proposals automatically
func (e *Engine) AnalyzeAndPropose(ctx context.Context, data *AnalysisData) ([]*Proposal, error) {
	var proposals []*Proposal

	// 1. Analyze cost data and generate cost_reduce proposals
	if data.CostData != nil {
		costProposals := e.analyzeCost(data.CostData)
		proposals = append(proposals, costProposals...)
	}

	// 2. Analyze SLO data and generate performance proposals
	if data.SLOData != nil {
		sloProposals := e.analyzeSLO(data.SLOData)
		proposals = append(proposals, sloProposals...)
	}

	// 3. Analyze model data and generate model_switch proposals
	if data.ModelData != nil {
		modelProposals := e.analyzeModels(data.ModelData)
		proposals = append(proposals, modelProposals...)
	}

	// Create all proposals
	for _, p := range proposals {
		if err := e.CreateProposal(ctx, p); err != nil {
			fmt.Printf("Failed to create proposal: %v\n", err)
			continue
		}
	}

	return proposals, nil
}

// analyzeCost analyzes cost data and generates cost reduction proposals
func (e *Engine) analyzeCost(data *CostAnalysisData) []*Proposal {
	var proposals []*Proposal

	// Skip if no meaningful data
	if data.TotalCost <= 0 || data.RequestCount <= 0 {
		return proposals
	}

	// Check if forecast is significantly higher than current
	if data.ForecastCost > data.TotalCost*1.5 && data.ForecastCost > 10 {
		proposals = append(proposals, &Proposal{
			Type:           ProposalTypeCostReduce,
			Title:          "成本增长预警 - 建议优化",
			Description:    fmt.Sprintf("预测月成本 ¥%.2f 显著高于当前 ¥%.2f，建议审查高成本模型使用", data.ForecastCost, data.TotalCost),
			ExpectedBenefit: data.ForecastCost - data.TotalCost,
			RiskLevel:      "low",
			CurrentState:   fmt.Sprintf(`{"total_cost": %.2f, "forecast": %.2f}`, data.TotalCost, data.ForecastCost),
			ProposedState:  `{"action": "review_high_cost_models", "target_reduction": 0.2}`,
		})
	}

	// Check per-model costs for optimization
	for _, m := range data.ByModel {
		// High cost model - suggest optimization
		if m.Cost > data.TotalCost*0.3 && m.RequestCount > 100 {
			proposals = append(proposals, &Proposal{
				AgentID:         "",
				Type:           ProposalTypeCostReduce,
				Title:          fmt.Sprintf("高成本模型优化: %s", m.ModelName),
				Description:    fmt.Sprintf("模型 %s 占总成本 %.1f%% (¥%.2f)，建议评估是否有更经济的替代方案", m.ModelID, m.Cost/data.TotalCost*100, m.Cost),
				ExpectedBenefit: m.Cost * 0.3, // Estimate 30% savings
				RiskLevel:      "medium",
				CurrentState:   fmt.Sprintf(`{"model": "%s", "cost": %.2f, "requests": %d}`, m.ModelID, m.Cost, m.RequestCount),
				ProposedState:  fmt.Sprintf(`{"action": "evaluate_alternatives", "current_model": "%s"}`, m.ModelID),
			})
		}

		// High input price - suggest caching
		if m.InputPrice > 5.0 && m.RequestCount > 50 {
			proposals = append(proposals, &Proposal{
				Type:           ProposalTypeCostReduce,
				Title:          fmt.Sprintf("启用缓存优化: %s", m.ModelName),
				Description:    fmt.Sprintf("模型 %s 输入价格 $%.2f/1M tokens，建议启用提示词缓存减少重复计算", m.ModelID, m.InputPrice),
				ExpectedBenefit: m.Cost * 0.2,
				RiskLevel:      "low",
				CurrentState:   fmt.Sprintf(`{"model": "%s", "input_price": %.2f}`, m.ModelID, m.InputPrice),
				ProposedState:  `{"action": "enable_cache", "cache_ttl": 3600}`,
			})
		}
	}

	// High avg cost per request - suggest batch processing
	if data.AvgCostPerRequest > 0.1 {
		proposals = append(proposals, &Proposal{
			Type:           ProposalTypeCostReduce,
			Title:          "批处理优化建议",
			Description:    fmt.Sprintf("平均每请求成本 ¥%.4f 较高，建议启用批处理模式", data.AvgCostPerRequest),
			ExpectedBenefit: data.TotalCost * 0.15,
			RiskLevel:      "low",
			CurrentState:   fmt.Sprintf(`{"avg_cost_per_request": %.4f}`, data.AvgCostPerRequest),
			ProposedState:  `{"action": "enable_batch", "batch_size": 10}`,
		})
	}

	return proposals
}

// analyzeSLO analyzes SLO data and generates performance proposals
func (e *Engine) analyzeSLO(data *SLOAnalysisData) []*Proposal {
	var proposals []*Proposal

	for _, slo := range data.SLOs {
		// SLO is being breached
		if slo.Status == "critical" || slo.Status == "warning" {
			proposals = append(proposals, &Proposal{
				AgentID:         slo.AgentID,
				Type:           ProposalTypePerformance,
				Title:          fmt.Sprintf("SLO 修复: %s", slo.Name),
				Description:    fmt.Sprintf("SLO %s 当前值 %.2f%% 低于目标 %.2f%%，状态: %s", slo.Name, slo.Current*100, slo.Target*100, slo.Status),
				ExpectedBenefit: (slo.Target - slo.Current) * 100,
				RiskLevel:      "high",
				CurrentState:   fmt.Sprintf(`{"slo_id": "%s", "current": %.4f, "target": %.4f}`, slo.ID, slo.Current, slo.Target),
				ProposedState:  `{"action": "optimize_latency", "enable_caching": true, "reduce_retries": true}`,
			})
		}

		// High burn rate
		if slo.BurnRate > 2.0 {
			proposals = append(proposals, &Proposal{
				AgentID:         slo.AgentID,
				Type:           ProposalTypePerformance,
				Title:          fmt.Sprintf("高燃尽率警告: %s", slo.Name),
				Description:    fmt.Sprintf("SLO %s 燃尽率 %.2f 过高，错误预算消耗速度异常，建议立即排查", slo.Name, slo.BurnRate),
				ExpectedBenefit: slo.ErrorBudget * 100,
				RiskLevel:      "high",
				CurrentState:   fmt.Sprintf(`{"slo_id": "%s", "burn_rate": %.2f, "error_budget": %.4f}`, slo.ID, slo.BurnRate, slo.ErrorBudget),
				ProposedState:  `{"action": "reduce_burn_rate", "enable_circuit_breaker": true}`,
			})
		}

		// Low error budget
		if slo.ErrorBudget < 0.2 && slo.ErrorBudget > 0 {
			proposals = append(proposals, &Proposal{
				AgentID:         slo.AgentID,
				Type:           ProposalTypePerformance,
				Title:          fmt.Sprintf("错误预算不足: %s", slo.Name),
				Description:    fmt.Sprintf("SLO %s 错误预算仅剩 %.1f%%，建议增加容错机制", slo.Name, slo.ErrorBudget*100),
				ExpectedBenefit: (1 - slo.ErrorBudget) * 100,
				RiskLevel:      "medium",
				CurrentState:   fmt.Sprintf(`{"slo_id": "%s", "error_budget": %.4f}`, slo.ID, slo.ErrorBudget),
				ProposedState:  `{"action": "increase_redundancy", "fallback_enabled": true}`,
			})
		}
	}

	return proposals
}

// analyzeModels analyzes model data and generates model switch proposals
func (e *Engine) analyzeModels(data *ModelAnalysisData) []*Proposal {
	var proposals []*Proposal

	if len(data.Alternatives) == 0 {
		return proposals
	}

	// Find best alternative (good quality + lower cost)
	for _, alt := range data.Alternatives {
		if alt.InputPrice <= 0 {
			continue
		}

		currentInputPrice := data.CurrentCost / 1000 // rough estimate
		savingsPercent := 0.0
		if currentInputPrice > 0 {
			savingsPercent = (currentInputPrice - alt.InputPrice) / currentInputPrice * 100
		}

		// Recommend switch if savings > 30% and quality acceptable
		if savingsPercent > 30 && alt.QualityScore >= 0.8 {
			proposals = append(proposals, &Proposal{
				Type:           ProposalTypeModelSwitch,
				Title:          fmt.Sprintf("模型切换建议: %s", alt.ModelName),
				Description:    fmt.Sprintf("从 %s 切换到 %s 可节省约 %.1f%% 成本，质量影响评估: %.0f%%", data.CurrentModel, alt.ModelName, savingsPercent, alt.QualityScore*100),
				ExpectedBenefit: data.CurrentCost * savingsPercent / 100,
				RiskLevel:      "medium",
				CurrentState:   fmt.Sprintf(`{"model": "%s", "cost": %.2f}`, data.CurrentModel, data.CurrentCost),
				ProposedState:  fmt.Sprintf(`{"model": "%s", "input_price": %.2f, "output_price": %.2f}`, alt.ModelID, alt.InputPrice, alt.OutputPrice),
			})
		}

		// Recommend for testing if quality is good but need validation
		if savingsPercent > 20 && alt.QualityScore >= 0.7 && alt.QualityScore < 0.8 {
			proposals = append(proposals, &Proposal{
				Type:           ProposalTypeABTest,
				Title:          fmt.Sprintf("A/B 测试建议: %s", alt.ModelName),
				Description:    fmt.Sprintf("建议对 %s 进行 A/B 测试验证效果，潜在节省 %.1f%%", alt.ModelName, savingsPercent),
				ExpectedBenefit: data.CurrentCost * savingsPercent / 100 * 0.5, // 50% of potential if test passes
				RiskLevel:      "low",
				CurrentState:   fmt.Sprintf(`{"model": "%s"}`, data.CurrentModel),
				ProposedState:  fmt.Sprintf(`{"test_model": "%s", "traffic_split": 0.1}`, alt.ModelID),
			})
		}
	}

	return proposals
}

