// Package coordinate provides orchestration quality management
package coordinate

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

// OrchestrationType represents the type of orchestration
type OrchestrationType string

const (
	OrchestrationSequential OrchestrationType = "sequential"
	OrchestrationParallel   OrchestrationType = "parallel"
	OrchestrationConditional OrchestrationType = "conditional"
	OrchestrationLoop       OrchestrationType = "loop"
)

// OrchestrationRun represents an orchestration execution
type OrchestrationRun struct {
	ID           string            `gorm:"primaryKey"`
	AgentID      string            `gorm:"index"`
	Type         OrchestrationType `gorm:"index"`
	Status       string            // "running", "completed", "failed"
	Steps        string            // JSON step definitions
	Results      string            // JSON step results
	Score        float64           // Quality score
	LatencyMs    int64             // Total latency
	TokenCount   int64             // Total tokens
	Cost         float64           // Total cost
	SuccessCount int               // Successful steps
	FailCount    int               // Failed steps
	StartedAt    time.Time
	EndedAt      *time.Time
	Metadata     string
}

// OrchestrationScore represents scoring for an orchestration
type OrchestrationScore struct {
	OverallScore    float64
	SuccessRate     float64
	Efficiency      float64 // Time efficiency
	CostEfficiency  float64 // Cost efficiency
	QualityScore    float64 // Output quality
	StepScores      map[string]float64
	Recommendations []string
}

// OrchestrationReport represents a report on orchestrations
type OrchestrationReport struct {
	PeriodStart       time.Time
	PeriodEnd         time.Time
	TotalRuns         int64
	SuccessfulRuns    int64
	FailedRuns        int64
	AvgScore          float64
	AvgLatencyMs      float64
	AvgCost           float64
	TotalTokens       int64
	ByType            map[OrchestrationType]*TypeStats
	ByAgent           map[string]*AgentStats
	TopPerformers     []*OrchestrationRun
	BottomPerformers  []*OrchestrationRun
}

// TypeStats represents statistics by orchestration type
type TypeStats struct {
	Type       OrchestrationType
	Count      int64
	AvgScore   float64
	AvgLatency float64
	SuccessRate float64
}

// AgentStats represents statistics by agent
type AgentStats struct {
	AgentID    string
	Count      int64
	AvgScore   float64
	AvgLatency float64
	SuccessRate float64
}

// Engine is the orchestration quality engine
type Engine struct {
	db   *gorm.DB
	runs map[string]*OrchestrationRun
	mu   sync.RWMutex
}

// NewEngine creates a new coordinate engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:   db,
		runs: make(map[string]*OrchestrationRun),
	}
	e.loadRuns()
	return e
}

// NewEngineMemory creates an in-memory coordinate engine
func NewEngineMemory() *Engine {
	return &Engine{
		runs: make(map[string]*OrchestrationRun),
	}
}

// loadRuns loads runs from database
func (e *Engine) loadRuns() {
	if e.db == nil {
		return
	}

	var runs []OrchestrationRun
	cutoff := time.Now().AddDate(0, 0, -7) // Last 7 days
	if err := e.db.Where("started_at > ?", cutoff).Find(&runs).Error; err != nil {
		return
	}

	for _, r := range runs {
		e.runs[r.ID] = &r
	}
}

// RecordOrchestration records an orchestration run
func (e *Engine) RecordOrchestration(ctx context.Context, run *OrchestrationRun) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	run.StartedAt = time.Now()
	run.Status = "running"

	if e.db != nil {
		if err := e.db.Create(run).Error; err != nil {
			return fmt.Errorf("record orchestration: %w", err)
		}
	}

	e.runs[run.ID] = run
	return nil
}

// CompleteOrchestration marks an orchestration as completed
func (e *Engine) CompleteOrchestration(ctx context.Context, id string, results string, score float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, exists := e.runs[id]
	if !exists {
		return fmt.Errorf("orchestration not found")
	}

	now := time.Now()
	run.Status = "completed"
	run.EndedAt = &now
	run.Results = results
	run.Score = score
	run.LatencyMs = now.Sub(run.StartedAt).Milliseconds()

	if e.db != nil {
		e.db.Model(run).Updates(map[string]interface{}{
			"status":    "completed",
			"ended_at":  now,
			"results":   results,
			"score":     score,
			"latency_ms": run.LatencyMs,
		})
	}

	return nil
}

// FailOrchestration marks an orchestration as failed
func (e *Engine) FailOrchestration(ctx context.Context, id, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, exists := e.runs[id]
	if !exists {
		return fmt.Errorf("orchestration not found")
	}

	now := time.Now()
	run.Status = "failed"
	run.EndedAt = &now

	var metadata map[string]interface{}
	if run.Metadata != "" {
		json.Unmarshal([]byte(run.Metadata), &metadata)
	}
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["failure_reason"] = reason
	data, _ := json.Marshal(metadata)
	run.Metadata = string(data)

	if e.db != nil {
		e.db.Model(run).Updates(map[string]interface{}{
			"status":   "failed",
			"ended_at": now,
			"metadata": run.Metadata,
		})
	}

	return nil
}

// ScoreOrchestration calculates quality score for an orchestration
func (e *Engine) ScoreOrchestration(ctx context.Context, id string) (*OrchestrationScore, error) {
	e.mu.RLock()
	run, exists := e.runs[id]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("orchestration not found")
	}

	score := &OrchestrationScore{
		StepScores: make(map[string]float64),
	}

	// Calculate success rate
	totalSteps := run.SuccessCount + run.FailCount
	if totalSteps > 0 {
		score.SuccessRate = float64(run.SuccessCount) / float64(totalSteps)
	}

	// Calculate efficiency (based on expected vs actual latency)
	// Assume expected latency is based on step count
	var steps []map[string]interface{}
	if run.Steps != "" {
		json.Unmarshal([]byte(run.Steps), &steps)
	}
	expectedLatency := float64(len(steps)) * 1000 // 1 second per step expected
	if expectedLatency > 0 {
		score.Efficiency = math.Min(1.0, expectedLatency/float64(run.LatencyMs))
	}

	// Calculate cost efficiency
	if run.Cost > 0 {
		// Lower cost is better
		score.CostEfficiency = 1 / (1 + run.Cost)
	}

	// Quality score from stored score
	score.QualityScore = run.Score

	// Calculate overall score (weighted average)
	score.OverallScore = score.SuccessRate*0.4 +
		score.Efficiency*0.2 +
		score.CostEfficiency*0.2 +
		score.QualityScore*0.2

	// Generate recommendations
	score.Recommendations = e.generateRecommendations(score, run)

	return score, nil
}

// generateRecommendations generates improvement recommendations
func (e *Engine) generateRecommendations(score *OrchestrationScore, run *OrchestrationRun) []string {
	var recommendations []string

	if score.SuccessRate < 0.9 {
		recommendations = append(recommendations, "Consider adding retry logic for failed steps")
	}
	if score.Efficiency < 0.7 {
		recommendations = append(recommendations, "Optimize step execution for better latency")
	}
	if score.CostEfficiency < 0.5 {
		recommendations = append(recommendations, "Review token usage for cost optimization")
	}
	if score.QualityScore < 0.7 {
		recommendations = append(recommendations, "Review output quality and adjust parameters")
	}

	return recommendations
}

// OrchestratorReport generates a report for orchestrations
func (e *Engine) OrchestratorReport(ctx context.Context, agentID string, start, end time.Time) (*OrchestrationReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	report := &OrchestrationReport{
		PeriodStart: start,
		PeriodEnd:   end,
		ByType:      make(map[OrchestrationType]*TypeStats),
		ByAgent:     make(map[string]*AgentStats),
	}

	var runs []*OrchestrationRun
	for _, r := range e.runs {
		if agentID != "" && r.AgentID != agentID {
			continue
		}
		if r.StartedAt.Before(start) || r.StartedAt.After(end) {
			continue
		}
		runs = append(runs, r)
	}

	// Calculate statistics
	for _, r := range runs {
		report.TotalRuns++
		report.TotalTokens += r.TokenCount

		if r.Status == "completed" {
			report.SuccessfulRuns++
		} else if r.Status == "failed" {
			report.FailedRuns++
		}

		report.AvgScore += r.Score
		report.AvgLatencyMs += float64(r.LatencyMs)
		report.AvgCost += r.Cost

		// By type
		if _, exists := report.ByType[r.Type]; !exists {
			report.ByType[r.Type] = &TypeStats{Type: r.Type}
		}
		ts := report.ByType[r.Type]
		ts.Count++
		ts.AvgScore += r.Score
		ts.AvgLatency += float64(r.LatencyMs)
		if r.Status == "completed" {
			ts.SuccessRate++
		}

		// By agent
		if _, exists := report.ByAgent[r.AgentID]; !exists {
			report.ByAgent[r.AgentID] = &AgentStats{AgentID: r.AgentID}
		}
		as := report.ByAgent[r.AgentID]
		as.Count++
		as.AvgScore += r.Score
		as.AvgLatency += float64(r.LatencyMs)
		if r.Status == "completed" {
			as.SuccessRate++
		}
	}

	// Calculate averages
	if report.TotalRuns > 0 {
		report.AvgScore /= float64(report.TotalRuns)
		report.AvgLatencyMs /= float64(report.TotalRuns)
		report.AvgCost /= float64(report.TotalRuns)
	}

	for _, ts := range report.ByType {
		if ts.Count > 0 {
			ts.AvgScore /= float64(ts.Count)
			ts.AvgLatency /= float64(ts.Count)
			ts.SuccessRate /= float64(ts.Count)
		}
	}

	for _, as := range report.ByAgent {
		if as.Count > 0 {
			as.AvgScore /= float64(as.Count)
			as.AvgLatency /= float64(as.Count)
			as.SuccessRate /= float64(as.Count)
		}
	}

	// Sort for top/bottom performers
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Score > runs[j].Score
	})

	if len(runs) > 5 {
		report.TopPerformers = runs[:5]
		report.BottomPerformers = runs[len(runs)-5:]
	} else {
		report.TopPerformers = runs
	}

	return report, nil
}

// OrchestratorRuns lists orchestration runs
func (e *Engine) OrchestratorRuns(ctx context.Context, agentID string, limit int) ([]*OrchestrationRun, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var runs []*OrchestrationRun
	for _, r := range e.runs {
		if agentID != "" && r.AgentID != agentID {
			continue
		}
		runs = append(runs, r)
	}

	// Sort by started_at
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})

	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}

	return runs, nil
}

// GetRun retrieves a specific run
func (e *Engine) GetRun(ctx context.Context, id string) (*OrchestrationRun, error) {
	e.mu.RLock()
	run, exists := e.runs[id]
	e.mu.RUnlock()

	if exists {
		return run, nil
	}

	if e.db != nil {
		var r OrchestrationRun
		if err := e.db.First(&r, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get run: %w", err)
		}
		return &r, nil
	}

	return nil, fmt.Errorf("run not found")
}
