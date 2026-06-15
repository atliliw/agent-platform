// Package planner provides plan evaluation functionality
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlanStatus represents the status of a plan
type PlanStatus string

const (
	PlanStatusDraft     PlanStatus = "draft"
	PlanStatusExecuting PlanStatus = "executing"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusFailed    PlanStatus = "failed"
)

// Plan represents an execution plan
type Plan struct {
	ID          string       `gorm:"primaryKey"`
	AgentID     string       `gorm:"index"`
	Goal        string
	Steps       string       // JSON step definitions
	Status      PlanStatus   `gorm:"index"`
	Score       float64      // Plan quality score
	ExecutionMs int64        // Execution time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ExecutedAt  *time.Time
}

// PlanStep represents a step in a plan
type PlanStep struct {
	ID          string
	Description string
	Type        string // "action", "decision", "observation"
	Dependencies []string
	Expected    string
	Actual      string
	Score       float64
	Status      string
}

// PlanScore represents scoring for a plan
type PlanScore struct {
	OverallScore    float64
	Completeness    float64 // How complete is the plan
	Feasibility     float64 // How feasible is the plan
	Efficiency      float64 // How efficient is the plan
	Robustness      float64 // How robust is the plan
	StepScores      map[string]float64
	Issues          []string
	Suggestions     []string
}

// EvalResult represents an evaluation result
type EvalResult struct {
	PlanID       string
	Score        float64
	Metrics      map[string]float64
	Issues       []string
	Recommendations []string
}

// Engine is the planner evaluation engine
type Engine struct {
	db    *gorm.DB
	plans map[string]*Plan
	mu    sync.RWMutex
}

// NewEngine creates a new planner engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:    db,
		plans: make(map[string]*Plan),
	}
	e.loadPlans()
	return e
}

// NewEngineMemory creates an in-memory planner engine
func NewEngineMemory() *Engine {
	return &Engine{
		plans: make(map[string]*Plan),
	}
}

// loadPlans loads plans from database
func (e *Engine) loadPlans() {
	if e.db == nil {
		return
	}

	var plans []Plan
	if err := e.db.Find(&plans).Error; err != nil {
		return
	}

	for _, p := range plans {
		e.plans[p.ID] = &p
	}
}

// CreatePlan creates a new plan
func (e *Engine) CreatePlan(ctx context.Context, plan *Plan) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if plan.ID == "" {
		plan.ID = uuid.New().String()
	}
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()
	plan.Status = PlanStatusDraft

	if e.db != nil {
		if err := e.db.Create(plan).Error; err != nil {
			return fmt.Errorf("create plan: %w", err)
		}
	}

	e.plans[plan.ID] = plan
	return nil
}

// GetPlan retrieves a plan
func (e *Engine) GetPlan(ctx context.Context, id string) (*Plan, error) {
	e.mu.RLock()
	plan, exists := e.plans[id]
	e.mu.RUnlock()

	if exists {
		return plan, nil
	}

	if e.db != nil {
		var p Plan
		if err := e.db.First(&p, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get plan: %w", err)
		}
		return &p, nil
	}

	return nil, fmt.Errorf("plan not found")
}

// ScorePlan evaluates and scores a plan
func (e *Engine) ScorePlan(ctx context.Context, planID string) (*PlanScore, error) {
	e.mu.RLock()
	plan, exists := e.plans[planID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plan not found")
	}

	score := &PlanScore{
		StepScores: make(map[string]float64),
	}

	// Parse steps
	var steps []PlanStep
	if plan.Steps != "" {
		if err := json.Unmarshal([]byte(plan.Steps), &steps); err != nil {
			return nil, fmt.Errorf("parse steps: %w", err)
		}
	}

	// Score completeness
	score.Completeness = e.scoreCompleteness(plan, steps)

	// Score feasibility
	score.Feasibility = e.scoreFeasibility(plan, steps)

	// Score efficiency
	score.Efficiency = e.scoreEfficiency(plan, steps)

	// Score robustness
	score.Robustness = e.scoreRobustness(plan, steps)

	// Calculate overall score
	score.OverallScore = score.Completeness*0.3 +
		score.Feasibility*0.3 +
		score.Efficiency*0.2 +
		score.Robustness*0.2

	// Score individual steps
	for _, step := range steps {
		stepScore := e.scoreStep(step)
		score.StepScores[step.ID] = stepScore
	}

	// Identify issues
	score.Issues = e.identifyIssues(plan, steps, score)

	// Generate suggestions
	score.Suggestions = e.generateSuggestions(plan, steps, score)

	// Update plan score
	e.mu.Lock()
	plan.Score = score.OverallScore
	plan.UpdatedAt = time.Now()
	if e.db != nil {
		e.db.Model(plan).Updates(map[string]interface{}{
			"score":      score.OverallScore,
			"updated_at": time.Now(),
		})
	}
	e.mu.Unlock()

	return score, nil
}

// scoreCompleteness scores plan completeness
func (e *Engine) scoreCompleteness(plan *Plan, steps []PlanStep) float64 {
	if len(steps) == 0 {
		return 0
	}

	// Check if plan has clear goal
	goalScore := 0.0
	if plan.Goal != "" && len(plan.Goal) > 10 {
		goalScore = 1.0
	}

	// Check if all steps have descriptions
	stepDescScore := 0.0
	for _, step := range steps {
		if step.Description != "" {
			stepDescScore++
		}
	}
	if len(steps) > 0 {
		stepDescScore /= float64(len(steps))
	}

	// Check if dependencies are satisfied
	depScore := 1.0
	stepIDs := make(map[string]bool)
	for _, step := range steps {
		stepIDs[step.ID] = true
	}
	for _, step := range steps {
		for _, dep := range step.Dependencies {
			if !stepIDs[dep] {
				depScore -= 0.1
			}
		}
	}
	depScore = math.Max(0, depScore)

	return goalScore*0.3 + stepDescScore*0.4 + depScore*0.3
}

// scoreFeasibility scores plan feasibility
func (e *Engine) scoreFeasibility(plan *Plan, steps []PlanStep) float64 {
	if len(steps) == 0 {
		return 0.5 // Neutral score for empty plan
	}

	// Check step types
	actionCount := 0
	decisionCount := 0
	observationCount := 0

	for _, step := range steps {
		switch step.Type {
		case "action":
			actionCount++
		case "decision":
			decisionCount++
		case "observation":
			observationCount++
		}
	}

	// Good plans have a mix of actions and observations
	total := float64(len(steps))
	actionRatio := float64(actionCount) / total
	observationRatio := float64(observationCount) / total
	decisionRatio := float64(decisionCount) / total

	// Ideal: ~60% actions, ~30% observations, ~10% decisions
	actionScore := 1 - math.Abs(actionRatio-0.6)
	observationScore := 1 - math.Abs(observationRatio-0.3)
	decisionScore := 1 - math.Abs(decisionRatio-0.1)

	return actionScore*0.5 + observationScore*0.3 + decisionScore*0.2
}

// scoreEfficiency scores plan efficiency
func (e *Engine) scoreEfficiency(plan *Plan, steps []PlanStep) float64 {
	if len(steps) == 0 {
		return 1.0
	}

	// Check for redundant steps
	uniqueDescs := make(map[string]bool)
	for _, step := range steps {
		uniqueDescs[step.Description] = true
	}
	redundancyScore := float64(len(uniqueDescs)) / float64(len(steps))

	// Check for appropriate step count
	// Too few steps = incomplete, too many = inefficient
	stepCountScore := 1.0
	if len(steps) < 3 {
		stepCountScore = float64(len(steps)) / 3
	} else if len(steps) > 10 {
		stepCountScore = 10 / float64(len(steps))
	}

	// Check dependency depth
	depthScore := 1.0
	avgDeps := 0.0
	for _, step := range steps {
		avgDeps += float64(len(step.Dependencies))
	}
	avgDeps /= float64(len(steps))
	if avgDeps > 2 {
		depthScore = 2 / avgDeps
	}

	return redundancyScore*0.4 + stepCountScore*0.3 + depthScore*0.3
}

// scoreRobustness scores plan robustness
func (e *Engine) scoreRobustness(plan *Plan, steps []PlanStep) float64 {
	if len(steps) == 0 {
		return 0
	}

	// Check for decision points
	decisionCount := 0
	for _, step := range steps {
		if step.Type == "decision" {
			decisionCount++
		}
	}
	decisionScore := math.Min(1.0, float64(decisionCount)/2) // At least 2 decisions for robustness

	// Check for expected outcomes
	expectedCount := 0
	for _, step := range steps {
		if step.Expected != "" {
			expectedCount++
		}
	}
	expectedScore := float64(expectedCount) / float64(len(steps))

	// Check for observation steps
	observationCount := 0
	for _, step := range steps {
		if step.Type == "observation" {
			observationCount++
		}
	}
	observationScore := math.Min(1.0, float64(observationCount)/2) // At least 2 observations

	return decisionScore*0.4 + expectedScore*0.3 + observationScore*0.3
}

// scoreStep scores an individual step
func (e *Engine) scoreStep(step PlanStep) float64 {
	score := 0.5 // Base score

	// Has description
	if step.Description != "" {
		score += 0.2
	}

	// Has type
	if step.Type != "" {
		score += 0.1
	}

	// Has expected outcome
	if step.Expected != "" {
		score += 0.1
	}

	// Status is completed
	if step.Status == "completed" {
		score += 0.1
	}

	return math.Min(1.0, score)
}

// identifyIssues identifies issues in the plan
func (e *Engine) identifyIssues(plan *Plan, steps []PlanStep, score *PlanScore) []string {
	var issues []string

	if score.Completeness < 0.5 {
		issues = append(issues, "Plan is incomplete")
	}
	if score.Feasibility < 0.5 {
		issues = append(issues, "Plan may not be feasible")
	}
	if score.Efficiency < 0.5 {
		issues = append(issues, "Plan is inefficient")
	}
	if score.Robustness < 0.5 {
		issues = append(issues, "Plan lacks robustness")
	}

	// Check for circular dependencies
	stepIDs := make(map[string]bool)
	for _, step := range steps {
		stepIDs[step.ID] = true
	}
	for _, step := range steps {
		for _, dep := range step.Dependencies {
			if dep == step.ID {
				issues = append(issues, "Circular dependency detected")
			}
		}
	}

	return issues
}

// generateSuggestions generates improvement suggestions
func (e *Engine) generateSuggestions(plan *Plan, steps []PlanStep, score *PlanScore) []string {
	var suggestions []string

	if score.Completeness < 0.7 {
		suggestions = append(suggestions, "Add more detailed step descriptions")
	}
	if score.Feasibility < 0.7 {
		suggestions = append(suggestions, "Review step types for better balance")
	}
	if score.Efficiency < 0.7 {
		suggestions = append(suggestions, "Consider reducing redundant steps")
	}
	if score.Robustness < 0.7 {
		suggestions = append(suggestions, "Add decision points for error handling")
		suggestions = append(suggestions, "Add observation steps for verification")
	}

	// Check for missing expected outcomes
	missingExpected := 0
	for _, step := range steps {
		if step.Type == "action" && step.Expected == "" {
			missingExpected++
		}
	}
	if missingExpected > 0 {
		suggestions = append(suggestions, "Add expected outcomes for action steps")
	}

	return suggestions
}

// Evaluate evaluates a plan and returns result
func (e *Engine) Evaluate(ctx context.Context, planID string) (*EvalResult, error) {
	score, err := e.ScorePlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	result := &EvalResult{
		PlanID: planID,
		Score:  score.OverallScore,
		Metrics: map[string]float64{
			"completeness": score.Completeness,
			"feasibility":  score.Feasibility,
			"efficiency":   score.Efficiency,
			"robustness":   score.Robustness,
		},
		Issues:          score.Issues,
		Recommendations: score.Suggestions,
	}

	return result, nil
}

// ListPlans lists plans
func (e *Engine) ListPlans(ctx context.Context, agentID string, status PlanStatus) ([]*Plan, error) {
	if e.db != nil {
		query := e.db.Model(&Plan{})
		if agentID != "" {
			query = query.Where("agent_id = ?", agentID)
		}
		if status != "" {
			query = query.Where("status = ?", status)
		}

		var plans []*Plan
		if err := query.Order("created_at DESC").Find(&plans).Error; err != nil {
			return nil, fmt.Errorf("list plans: %w", err)
		}
		return plans, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Plan
	for _, p := range e.plans {
		if agentID != "" && p.AgentID != agentID {
			continue
		}
		if status != "" && p.Status != status {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

// UpdatePlan updates a plan
func (e *Engine) UpdatePlan(ctx context.Context, plan *Plan) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	plan.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Save(plan).Error; err != nil {
			return fmt.Errorf("update plan: %w", err)
		}
	}

	e.plans[plan.ID] = plan
	return nil
}

// DeletePlan deletes a plan
func (e *Engine) DeletePlan(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Delete(&Plan{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete plan: %w", err)
		}
	}

	delete(e.plans, id)
	return nil
}
