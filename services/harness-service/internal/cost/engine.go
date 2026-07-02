// Package cost provides cost intelligence and optimization
package cost

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

// ModelPricing represents pricing for a model
type ModelPricing struct {
	ID                string    `gorm:"primaryKey"`
	ModelID           string    `gorm:"uniqueIndex"`
	ModelName         string
	Provider          string
	InputPricePer1M   float64   // Price per 1M input tokens
	OutputPricePer1M  float64   // Price per 1M output tokens
	Currency          string
	EffectiveFrom     time.Time
	EffectiveTo       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// UsageRecord represents a usage record
type UsageRecord struct {
	ID           string    `gorm:"primaryKey"`
	AgentID      string    `gorm:"index"`
	ModelID      string    `gorm:"index"`
	SessionID    string    `gorm:"index"`
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	Currency      string
	Timestamp    time.Time `gorm:"index"`
	Metadata     string    // JSON
}

// CostReport represents a cost report
type CostReport struct {
	PeriodStart      time.Time
	PeriodEnd        time.Time
	TotalCost        float64
	TotalInputTokens int64
	TotalOutputTokens int64
	RequestCount     int64
	ByAgent          map[string]*AgentCost
	ByModel          map[string]*ModelCost
	Currency         string
}

// AgentCost represents cost breakdown by agent
type AgentCost struct {
	AgentID          string
	TotalCost        float64
	InputTokens      int64
	OutputTokens     int64
	RequestCount     int64
	AvgCostPerRequest float64
	Trend            string // "up", "down", "stable"
}

// ModelCost represents cost breakdown by model
type ModelCost struct {
	ModelID          string
	ModelName        string
	TotalCost        float64
	InputTokens      int64
	OutputTokens     int64
	RequestCount     int64
	AvgCostPerRequest float64
	AvgLatencyMs     float64
}

// IdleAgent represents an idle agent detection
type IdleAgent struct {
	AgentID      string
	LastActive   time.Time
	IdleHours    float64
	RunningCost  float64  // Cost of keeping it running
	InstanceType string
	Recommendation string
}

// CostRecommendation represents a cost optimization recommendation
type CostRecommendation struct {
	Type          string // "model_switch", "idle_termination", "cache_optimization", "batch_processing"
	Priority      string // "high", "medium", "low"
	Title         string
	Description   string
	PotentialSavings float64 // Monthly savings
	AgentID       string
	ModelID       string
	CurrentCost   float64
	ProposedCost  float64
	Details       map[string]interface{}
}

// ModelComparison represents a model cost comparison
type ModelComparison struct {
	CurrentModel    string
	AlternativeModel string
	CurrentCost     float64
	AlternativeCost float64
	SavingsPercent  float64
	QualityImpact   string // "none", "low", "medium", "high"
	Recommendation  string
}

// Engine is the cost intelligence engine
type Engine struct {
	db      *gorm.DB
	pricing map[string]*ModelPricing
	usage   map[string][]*UsageRecord // agentID -> usage
	mu      sync.RWMutex
}

// NewEngine creates a new cost engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:      db,
		pricing: make(map[string]*ModelPricing),
		usage:   make(map[string][]*UsageRecord),
	}
	e.loadPricing()
	e.loadUsage()
	return e
}

// NewEngineMemory creates an in-memory cost engine
func NewEngineMemory() *Engine {
	return &Engine{
		pricing: make(map[string]*ModelPricing),
		usage:   make(map[string][]*UsageRecord),
	}
}

// loadPricing loads model pricing from database
func (e *Engine) loadPricing() {
	if e.db == nil {
		return
	}

	var pricing []ModelPricing
	if err := e.db.Find(&pricing).Error; err != nil {
		return
	}

	for _, p := range pricing {
		e.pricing[p.ModelID] = &p
	}

	// Add default pricing if empty
	if len(e.pricing) == 0 {
		defaultPricing := []ModelPricing{
			{ID: uuid.New().String(), ModelID: "qwen-plus", ModelName: "Qwen Plus", Provider: "Alibaba", InputPricePer1M: 0.8, OutputPricePer1M: 2.0, Currency: "CNY"},
			{ID: uuid.New().String(), ModelID: "qwen-turbo", ModelName: "Qwen Turbo", Provider: "Alibaba", InputPricePer1M: 0.3, OutputPricePer1M: 0.6, Currency: "CNY"},
			{ID: uuid.New().String(), ModelID: "qwen-max", ModelName: "Qwen Max", Provider: "Alibaba", InputPricePer1M: 2.0, OutputPricePer1M: 6.0, Currency: "CNY"},
			{ID: uuid.New().String(), ModelID: "gpt-4", ModelName: "GPT-4", Provider: "OpenAI", InputPricePer1M: 30.0, OutputPricePer1M: 60.0, Currency: "USD"},
			{ID: uuid.New().String(), ModelID: "gpt-3.5-turbo", ModelName: "GPT-3.5 Turbo", Provider: "OpenAI", InputPricePer1M: 0.5, OutputPricePer1M: 1.5, Currency: "USD"},
			{ID: uuid.New().String(), ModelID: "claude-3-sonnet", ModelName: "Claude 3 Sonnet", Provider: "Anthropic", InputPricePer1M: 3.0, OutputPricePer1M: 15.0, Currency: "USD"},
		}
		for _, p := range defaultPricing {
			e.pricing[p.ModelID] = &p
		}
	}
}

// loadUsage loads usage records from database
func (e *Engine) loadUsage() {
	if e.db == nil {
		return
	}

	var usage []UsageRecord
	cutoff := time.Now().AddDate(0, 0, -30) // Last 30 days
	if err := e.db.Where("timestamp > ?", cutoff).Find(&usage).Error; err != nil {
		return
	}

	for _, u := range usage {
		e.usage[u.AgentID] = append(e.usage[u.AgentID], &u)
	}
}

// SetModelPricing sets pricing for a model
func (e *Engine) SetModelPricing(ctx context.Context, pricing *ModelPricing) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if pricing.ID == "" {
		pricing.ID = uuid.New().String()
	}
	pricing.CreatedAt = time.Now()
	pricing.UpdatedAt = time.Now()
	pricing.EffectiveFrom = time.Now()

	if e.db != nil {
		if err := e.db.Create(pricing).Error; err != nil {
			return fmt.Errorf("set model pricing: %w", err)
		}
	}

	e.pricing[pricing.ModelID] = pricing
	return nil
}

// ListModelPricing lists all model pricing
func (e *Engine) ListModelPricing(ctx context.Context) ([]*ModelPricing, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*ModelPricing
	for _, p := range e.pricing {
		result = append(result, p)
	}

	// Sort by model name
	sort.Slice(result, func(i, j int) bool {
		return result[i].ModelName < result[j].ModelName
	})

	return result, nil
}

// GetModelPricing gets pricing for a specific model
func (e *Engine) GetModelPricing(ctx context.Context, modelID string) (*ModelPricing, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, exists := e.pricing[modelID]
	if !exists {
		return nil, fmt.Errorf("model pricing not found: %s", modelID)
	}
	return p, nil
}

// CalculateCost calculates cost for given token usage
func (e *Engine) CalculateCost(modelID string, inputTokens, outputTokens int64) float64 {
	e.mu.RLock()
	pricing, exists := e.pricing[modelID]
	e.mu.RUnlock()

	if !exists {
		// Default pricing
		return float64(inputTokens+outputTokens) * 0.000001
	}

	inputCost := float64(inputTokens) * pricing.InputPricePer1M / 1000000
	outputCost := float64(outputTokens) * pricing.OutputPricePer1M / 1000000
	return inputCost + outputCost
}

// RecordUsage records a usage event
func (e *Engine) RecordUsage(ctx context.Context, agentID, modelID, sessionID string, inputTokens, outputTokens int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cost := e.CalculateCost(modelID, inputTokens, outputTokens)

	record := &UsageRecord{
		ID:           uuid.New().String(),
		AgentID:      agentID,
		ModelID:      modelID,
		SessionID:    sessionID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Currency:     "CNY",
		Timestamp:    time.Now(),
	}

	if e.db != nil {
		if err := e.db.Create(record).Error; err != nil {
			return fmt.Errorf("record usage: %w", err)
		}
	}

	e.usage[agentID] = append(e.usage[agentID], record)
	return nil
}

// CostReport generates a cost report for a time period
func (e *Engine) CostReport(ctx context.Context, agentID string, start, end time.Time) (*CostReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fmt.Printf("[CostReport] Generating report for agent=%s start=%v end=%v\n", agentID, start, end)
	fmt.Printf("[CostReport] Total agents in usage map: %d\n", len(e.usage))
	for aID, records := range e.usage {
		fmt.Printf("[CostReport] Agent %s has %d records\n", aID, len(records))
	}

	report := &CostReport{
		PeriodStart: start,
		PeriodEnd:   end,
		ByAgent:     make(map[string]*AgentCost),
		ByModel:     make(map[string]*ModelCost),
		Currency:    "CNY",
	}

	// Aggregate usage
	for aID, records := range e.usage {
		if agentID != "" && aID != agentID {
			continue
		}

		for _, r := range records {
			fmt.Printf("[CostReport] Checking record: timestamp=%v cost=%.6f\n", r.Timestamp, r.Cost)
			if r.Timestamp.Before(start) || r.Timestamp.After(end) {
				fmt.Printf("[CostReport] Record filtered out by time range\n")
				continue
			}

			report.TotalCost += r.Cost
			report.TotalInputTokens += r.InputTokens
			report.TotalOutputTokens += r.OutputTokens
			report.RequestCount++

			// By agent
			if _, exists := report.ByAgent[aID]; !exists {
				report.ByAgent[aID] = &AgentCost{AgentID: aID}
			}
			ac := report.ByAgent[aID]
			ac.TotalCost += r.Cost
			ac.InputTokens += r.InputTokens
			ac.OutputTokens += r.OutputTokens
			ac.RequestCount++

			// By model
			if _, exists := report.ByModel[r.ModelID]; !exists {
				p := e.pricing[r.ModelID]
				modelName := r.ModelID
				if p != nil {
					modelName = p.ModelName
				}
				report.ByModel[r.ModelID] = &ModelCost{
					ModelID:   r.ModelID,
					ModelName: modelName,
				}
			}
			mc := report.ByModel[r.ModelID]
			mc.TotalCost += r.Cost
			mc.InputTokens += r.InputTokens
			mc.OutputTokens += r.OutputTokens
			mc.RequestCount++
		}
	}

	// Calculate averages
	for _, ac := range report.ByAgent {
		if ac.RequestCount > 0 {
			ac.AvgCostPerRequest = ac.TotalCost / float64(ac.RequestCount)
		}
	}
	for _, mc := range report.ByModel {
		if mc.RequestCount > 0 {
			mc.AvgCostPerRequest = mc.TotalCost / float64(mc.RequestCount)
		}
	}

	return report, nil
}

// IdleAgents detects idle agents
func (e *Engine) IdleAgents(ctx context.Context, idleThreshold time.Duration) ([]*IdleAgent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var idleAgents []*IdleAgent

	for agentID, records := range e.usage {
		if len(records) == 0 {
			continue
		}

		// Sort by timestamp
		sort.Slice(records, func(i, j int) bool {
			return records[i].Timestamp.After(records[j].Timestamp)
		})

		lastActive := records[0].Timestamp
		idleTime := time.Since(lastActive)

		if idleTime > idleThreshold {
			// Calculate running cost (estimated)
			avgCostPerDay := e.calculateDailyCost(records)
			runningCost := avgCostPerDay * idleTime.Hours() / 24

			idle := &IdleAgent{
				AgentID:       agentID,
				LastActive:    lastActive,
				IdleHours:     idleTime.Hours(),
				RunningCost:   runningCost,
				Recommendation: "Consider terminating or optimizing idle agent",
			}
			idleAgents = append(idleAgents, idle)
		}
	}

	// Sort by idle hours (most idle first)
	sort.Slice(idleAgents, func(i, j int) bool {
		return idleAgents[i].IdleHours > idleAgents[j].IdleHours
	})

	return idleAgents, nil
}

// calculateDailyCost calculates average daily cost from records
func (e *Engine) calculateDailyCost(records []*UsageRecord) float64 {
	if len(records) == 0 {
		return 0
	}

	var totalCost float64
	var oldest, newest time.Time

	for i, r := range records {
		totalCost += r.Cost
		if i == 0 {
			oldest = r.Timestamp
			newest = r.Timestamp
		} else {
			if r.Timestamp.Before(oldest) {
				oldest = r.Timestamp
			}
			if r.Timestamp.After(newest) {
				newest = r.Timestamp
			}
		}
	}

	days := newest.Sub(oldest).Hours() / 24
	if days < 1 {
		days = 1
	}

	return totalCost / days
}

// ModelComparison compares model costs
func (e *Engine) ModelComparison(ctx context.Context, currentModel string) ([]*ModelComparison, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	currentPricing, exists := e.pricing[currentModel]
	if !exists {
		return nil, fmt.Errorf("current model pricing not found")
	}

	var comparisons []*ModelComparison

	for modelID, pricing := range e.pricing {
		if modelID == currentModel {
			continue
		}

		// Calculate average cost for comparison
		avgInputTokens := int64(1000)
		avgOutputTokens := int64(500)

		currentCost := (float64(avgInputTokens)*currentPricing.InputPricePer1M + float64(avgOutputTokens)*currentPricing.OutputPricePer1M) / 1000000
		altCost := (float64(avgInputTokens)*pricing.InputPricePer1M + float64(avgOutputTokens)*pricing.OutputPricePer1M) / 1000000

		savingsPercent := 0.0
		if currentCost > 0 {
			savingsPercent = (currentCost - altCost) / currentCost * 100
		}

		qualityImpact := "none"
		if currentPricing.InputPricePer1M > pricing.InputPricePer1M*2 {
			qualityImpact = "high"
		} else if currentPricing.InputPricePer1M > pricing.InputPricePer1M*1.5 {
			qualityImpact = "medium"
		} else if currentPricing.InputPricePer1M > pricing.InputPricePer1M {
			qualityImpact = "low"
		}

		recommendation := ""
		if savingsPercent > 50 && qualityImpact == "low" {
			recommendation = "Highly recommended switch"
		} else if savingsPercent > 30 && qualityImpact != "high" {
			recommendation = "Consider switching"
		} else if savingsPercent > 0 && qualityImpact == "none" {
			recommendation = "Can switch for cost savings"
		}

		comparison := &ModelComparison{
			CurrentModel:    currentModel,
			AlternativeModel: modelID,
			CurrentCost:     currentCost,
			AlternativeCost: altCost,
			SavingsPercent:  savingsPercent,
			QualityImpact:   qualityImpact,
			Recommendation:  recommendation,
		}
		comparisons = append(comparisons, comparison)
	}

	// Sort by savings percent
	sort.Slice(comparisons, func(i, j int) bool {
		return comparisons[i].SavingsPercent > comparisons[j].SavingsPercent
	})

	return comparisons, nil
}

// Recommendations generates cost optimization recommendations
func (e *Engine) Recommendations(ctx context.Context) ([]*CostRecommendation, error) {
	var recommendations []*CostRecommendation

	// Check for idle agents
	idleAgents, err := e.IdleAgents(ctx, 24*time.Hour)
	if err == nil {
		for _, idle := range idleAgents {
			rec := &CostRecommendation{
				Type:            "idle_termination",
				Priority:        "high",
				Title:           fmt.Sprintf("Idle agent: %s", idle.AgentID),
				Description:     fmt.Sprintf("Agent has been idle for %.1f hours", idle.IdleHours),
				PotentialSavings: idle.RunningCost * 30, // Monthly savings
				AgentID:         idle.AgentID,
				CurrentCost:     idle.RunningCost,
				ProposedCost:    0,
			}
			recommendations = append(recommendations, rec)
		}
	}

	// Check for expensive model usage
	e.mu.RLock()
	modelUsage := make(map[string]*ModelCost)
	for _, records := range e.usage {
		for _, r := range records {
			if _, exists := modelUsage[r.ModelID]; !exists {
				modelUsage[r.ModelID] = &ModelCost{
					ModelID: r.ModelID,
				}
			}
			mc := modelUsage[r.ModelID]
			mc.TotalCost += r.Cost
			mc.RequestCount++
		}
	}

	// Check for models with cheaper alternatives
	for modelID, mc := range modelUsage {
		comparisons, _ := e.ModelComparison(ctx, modelID)
		for _, comp := range comparisons {
			if comp.SavingsPercent > 30 && comp.QualityImpact != "high" {
				rec := &CostRecommendation{
					Type:            "model_switch",
					Priority:        "medium",
					Title:           fmt.Sprintf("Switch from %s to %s", modelID, comp.AlternativeModel),
					Description:     fmt.Sprintf("Potential savings: %.1f%%", comp.SavingsPercent),
					PotentialSavings: mc.TotalCost * comp.SavingsPercent / 100 * 30, // Monthly
					ModelID:         modelID,
					CurrentCost:     mc.TotalCost,
					ProposedCost:    mc.TotalCost * (1 - comp.SavingsPercent/100),
				}
				recommendations = append(recommendations, rec)
				break // Only one recommendation per model
			}
		}
	}
	e.mu.RUnlock()

	// Check for caching opportunities
	for agentID, records := range e.usage {
		if len(records) < 100 {
			continue
		}

		// Check for repeated patterns
		patternCount := make(map[string]int)
		for _, r := range records {
			key := fmt.Sprintf("%s-%d-%d", r.ModelID, r.InputTokens/100, r.OutputTokens/100)
			patternCount[key]++
		}

		maxPattern := 0
		for _, count := range patternCount {
			if count > maxPattern {
				maxPattern = count
			}
		}

		if maxPattern > 10 {
			rec := &CostRecommendation{
				Type:        "cache_optimization",
				Priority:    "medium",
				Title:       fmt.Sprintf("Cache opportunities for agent %s", agentID),
				Description: "Repeated query patterns detected - consider implementing caching",
				AgentID:     agentID,
			}
			recommendations = append(recommendations, rec)
		}
	}

	// Sort by priority and potential savings
	sort.Slice(recommendations, func(i, j int) bool {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		if priorityOrder[recommendations[i].Priority] != priorityOrder[recommendations[j].Priority] {
			return priorityOrder[recommendations[i].Priority] < priorityOrder[recommendations[j].Priority]
		}
		return recommendations[i].PotentialSavings > recommendations[j].PotentialSavings
	})

	return recommendations, nil
}

// GetCostTrend gets cost trend for an agent
func (e *Engine) GetCostTrend(ctx context.Context, agentID string, days int) ([]float64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	records := e.usage[agentID]
	if len(records) == 0 {
		return nil, nil
	}

	// Calculate daily costs
	dailyCosts := make(map[string]float64)
	for _, r := range records {
		date := r.Timestamp.Format("2006-01-02")
		dailyCosts[date] += r.Cost
	}

	// Generate trend for last N days
	var trend []float64
	now := time.Now()
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		trend = append(trend, dailyCosts[date])
	}

	return trend, nil
}

// ForecastCost forecasts cost for the next period
func (e *Engine) ForecastCost(ctx context.Context, agentID string, days int) (float64, error) {
	trend, err := e.GetCostTrend(ctx, agentID, 7)
	if err != nil {
		return 0, err
	}

	if len(trend) == 0 {
		return 0, nil
	}

	// Simple linear extrapolation
	var sum float64
	for _, c := range trend {
		sum += c
	}
	avgDaily := sum / float64(len(trend))

	// Apply growth factor if trend is increasing
	if len(trend) >= 3 {
		recentAvg := (trend[len(trend)-1] + trend[len(trend)-2] + trend[len(trend)-3]) / 3
		olderAvg := (trend[0] + trend[1] + trend[2]) / 3
		if olderAvg > 0 {
			growthFactor := recentAvg / olderAvg
			if growthFactor > 1 {
				avgDaily *= math.Min(growthFactor, 1.5) // Cap at 50% growth
			}
		}
	}

	return avgDaily * float64(days), nil
}

// ExportReport exports a cost report as JSON
func (e *Engine) ExportReport(ctx context.Context, report *CostReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("export report: %w", err)
	}
	return string(data), nil
}

// DeleteModelPricing deletes model pricing
func (e *Engine) DeleteModelPricing(ctx context.Context, modelID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Delete(&ModelPricing{}, "model_id = ?", modelID).Error; err != nil {
			return fmt.Errorf("delete model pricing: %w", err)
		}
	}

	delete(e.pricing, modelID)
	return nil
}

// GetTotalCost returns total cost for a period
func (e *Engine) GetTotalCost(ctx context.Context, start, end time.Time) (float64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var total float64
	for _, records := range e.usage {
		for _, r := range records {
			if r.Timestamp.After(start) && r.Timestamp.Before(end) {
				total += r.Cost
			}
		}
	}
	return total, nil
}

// ================== Real-time Data Collection ==================

// RecordLLMCall records an LLM call from metrics (called automatically by metrics callback)
func (e *Engine) RecordLLMCall(ctx context.Context, agentID, modelID string, inputTokens, outputTokens int64, cost float64, latencyMs int64, success bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	record := &UsageRecord{
		ID:           uuid.New().String(),
		AgentID:      agentID,
		ModelID:      modelID,
		SessionID:    agentID + "-" + time.Now().Format("20060102"),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Currency:     "CNY",
		Timestamp:    time.Now(),
	}

	// Build metadata
	metadata := map[string]interface{}{
		"latency_ms": latencyMs,
		"success":    success,
	}
	if metaData, err := json.Marshal(metadata); err == nil {
		record.Metadata = string(metaData)
	}

	if e.db != nil {
		if err := e.db.Create(record).Error; err != nil {
			return fmt.Errorf("record llm call: %w", err)
		}
	}

	e.usage[agentID] = append(e.usage[agentID], record)

	// Keep only last 1000 records per agent in memory
	if len(e.usage[agentID]) > 1000 {
		e.usage[agentID] = e.usage[agentID][len(e.usage[agentID])-1000:]
	}

	return nil
}

// GetAgentMetrics returns current metrics for an agent
func (e *Engine) GetAgentMetrics(ctx context.Context, agentID string, lookback time.Duration) *AgentMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-lookback)

	metrics := &AgentMetrics{
		AgentID: agentID,
	}

	records := e.usage[agentID]
	for _, r := range records {
		if r.Timestamp.Before(cutoff) {
			continue
		}

		metrics.TotalCalls++
		metrics.TotalCost += r.Cost
		metrics.TotalInputTokens += r.InputTokens
		metrics.TotalOutputTokens += r.OutputTokens

		// Track by model
		if metrics.ByModel == nil {
			metrics.ByModel = make(map[string]*ModelCost)
		}
		if _, exists := metrics.ByModel[r.ModelID]; !exists {
			metrics.ByModel[r.ModelID] = &ModelCost{
				ModelID:   r.ModelID,
				ModelName: r.ModelID,
			}
		}
		mc := metrics.ByModel[r.ModelID]
		mc.TotalCost += r.Cost
		mc.InputTokens += r.InputTokens
		mc.OutputTokens += r.OutputTokens
		mc.RequestCount++
	}

	if metrics.TotalCalls > 0 {
		metrics.AvgCostPerCall = metrics.TotalCost / float64(metrics.TotalCalls)
	}

	return metrics
}

// AgentMetrics represents aggregated metrics for an agent
type AgentMetrics struct {
	AgentID          string
	TotalCalls       int64
	TotalCost        float64
	TotalInputTokens int64
	TotalOutputTokens int64
	AvgCostPerCall   float64
	ByModel          map[string]*ModelCost
}

// GetTopExpensiveAgents returns agents sorted by cost (descending)
func (e *Engine) GetTopExpensiveAgents(ctx context.Context, limit int, lookback time.Duration) []*AgentMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-lookback)

	// Aggregate per agent
	agentMetrics := make(map[string]*AgentMetrics)
	for agentID, records := range e.usage {
		metrics := &AgentMetrics{AgentID: agentID}
		for _, r := range records {
			if r.Timestamp.Before(cutoff) {
				continue
			}
			metrics.TotalCalls++
			metrics.TotalCost += r.Cost
			metrics.TotalInputTokens += r.InputTokens
			metrics.TotalOutputTokens += r.OutputTokens
		}
		if metrics.TotalCalls > 0 {
			metrics.AvgCostPerCall = metrics.TotalCost / float64(metrics.TotalCalls)
			agentMetrics[agentID] = metrics
		}
	}

	// Sort by cost
	var result []*AgentMetrics
	for _, m := range agentMetrics {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCost > result[j].TotalCost
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// GetCostBreakdown returns cost breakdown by model for a time period
func (e *Engine) GetCostBreakdown(ctx context.Context, start, end time.Time) map[string]*ModelCost {
	e.mu.RLock()
	defer e.mu.RUnlock()

	breakdown := make(map[string]*ModelCost)
	for _, records := range e.usage {
		for _, r := range records {
			if r.Timestamp.Before(start) || r.Timestamp.After(end) {
				continue
			}

			if _, exists := breakdown[r.ModelID]; !exists {
				p := e.pricing[r.ModelID]
				modelName := r.ModelID
				if p != nil {
					modelName = p.ModelName
				}
				breakdown[r.ModelID] = &ModelCost{
					ModelID:   r.ModelID,
					ModelName: modelName,
				}
			}
			mc := breakdown[r.ModelID]
			mc.TotalCost += r.Cost
			mc.InputTokens += r.InputTokens
			mc.OutputTokens += r.OutputTokens
			mc.RequestCount++
		}
	}

	// Calculate averages
	for _, mc := range breakdown {
		if mc.RequestCount > 0 {
			mc.AvgCostPerRequest = mc.TotalCost / float64(mc.RequestCount)
		}
	}

	return breakdown
}

// GetStats returns cost engine statistics
func (e *Engine) GetStats(ctx context.Context) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var totalRecords int64
	var totalCost float64
	agentCount := len(e.usage)

	for _, records := range e.usage {
		totalRecords += int64(len(records))
		for _, r := range records {
			totalCost += r.Cost
		}
	}

	return map[string]interface{}{
		"total_agents":    agentCount,
		"total_records":   totalRecords,
		"total_cost":      totalCost,
		"models_priced":   len(e.pricing),
	}
}
