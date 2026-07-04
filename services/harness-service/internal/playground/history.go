// Package playground provides Prompt Playground history management
package playground

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"agent-platform/pkg/llm"
)

// PlaygroundHistory represents a playground execution history record
type PlaygroundHistory struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	UserID      string                 `json:"user_id"`
	SessionID   string                 `json:"session_id"`
	Model       string                 `json:"model"`
	Messages    []llm.Message          `json:"messages"`
	Result      *PlaygroundResult      `json:"result,omitempty"`
	Comparison  *CompareModelsResponse `json:"comparison,omitempty"`
	Temperature float64                `json:"temperature"`
	MaxTokens   int                    `json:"max_tokens"`
	TopP        float64                `json:"top_p"`
	Parameters  map[string]interface{} `json:"parameters"`
	Streamed    bool                   `json:"streamed"`
	CreatedAt   time.Time              `json:"created_at"`
}

// Recorder manages playground history storage
type Recorder struct {
	histories map[string]*PlaygroundHistory
	mu        sync.RWMutex
}

// NewRecorder creates a new playground history recorder
func NewRecorder() *Recorder {
	return &Recorder{
		histories: make(map[string]*PlaygroundHistory),
	}
}

// Save saves a playground history record
func (r *Recorder) Save(ctx context.Context, history *PlaygroundHistory) error {
	if history == nil {
		return fmt.Errorf("history cannot be nil")
	}

	if history.ID == "" {
		return fmt.Errorf("history ID required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.histories[history.ID] = history

	fmt.Printf("[Playground] Saved history: id=%s model=%s user=%s\n",
		history.ID, history.Model, history.UserID)

	return nil
}

// Get retrieves a specific playground history by ID
func (r *Recorder) Get(ctx context.Context, historyID string) (*PlaygroundHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	history, ok := r.histories[historyID]
	if !ok {
		return nil, fmt.Errorf("history not found: %s", historyID)
	}

	return history, nil
}

// List retrieves playground histories filtered by tenant and user
func (r *Recorder) List(ctx context.Context, tenantID, userID string, limit int) ([]*PlaygroundHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*PlaygroundHistory

	for _, history := range r.histories {
		// Filter by tenant
		if tenantID != "" && history.TenantID != tenantID {
			continue
		}

		// Filter by user
		if userID != "" && history.UserID != userID {
			continue
		}

		results = append(results, history)
	}

	// Sort by created_at descending (newest first)
	sortByCreatedAtDesc(results)

	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Delete deletes a playground history by ID
func (r *Recorder) Delete(ctx context.Context, historyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.histories[historyID]
	if !ok {
		return fmt.Errorf("history not found: %s", historyID)
	}

	delete(r.histories, historyID)

	fmt.Printf("[Playground] Deleted history: id=%s\n", historyID)

	return nil
}

// DeleteByUser deletes all histories for a specific user
func (r *Recorder) DeleteByUser(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var deletedCount int

	for id, history := range r.histories {
		if history.UserID == userID {
			delete(r.histories, id)
			deletedCount++
		}
	}

	fmt.Printf("[Playground] Deleted %d histories for user: %s\n", deletedCount, userID)

	return nil
}

// DeleteByTenant deletes all histories for a specific tenant
func (r *Recorder) DeleteByTenant(ctx context.Context, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var deletedCount int

	for id, history := range r.histories {
		if history.TenantID == tenantID {
			delete(r.histories, id)
			deletedCount++
		}
	}

	fmt.Printf("[Playground] Deleted %d histories for tenant: %s\n", deletedCount, tenantID)

	return nil
}

// Clear clears all histories (use with caution)
func (r *Recorder) Clear(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := len(r.histories)
	r.histories = make(map[string]*PlaygroundHistory)

	fmt.Printf("[Playground] Cleared all %d histories\n", count)

	return nil
}

// Count returns the total number of histories
func (r *Recorder) Count(ctx context.Context) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.histories)
}

// CountByUser returns the number of histories for a specific user
func (r *Recorder) CountByUser(ctx context.Context, userID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	for _, history := range r.histories {
		if history.UserID == userID {
			count++
		}
	}

	return count
}

// GetStats returns statistics about playground usage
func (r *Recorder) GetStats(ctx context.Context, tenantID string) *RecorderStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &RecorderStats{
		TenantID: tenantID,
	}

	for _, history := range r.histories {
		if tenantID != "" && history.TenantID != tenantID {
			continue
		}

		stats.TotalExecutions++

		if history.Streamed {
			stats.StreamedExecutions++
		}

		if history.Comparison != nil {
			stats.ComparisonExecutions++
		}

		if history.Result != nil {
			stats.TotalTokens += history.Result.TotalTokens
			stats.TotalCost += history.Result.Cost
			stats.TotalLatency += history.Result.Latency

			// Track model usage
			if stats.ModelCounts == nil {
				stats.ModelCounts = make(map[string]int)
			}
			stats.ModelCounts[history.Model]++
		}
	}

	// Calculate averages
	if stats.TotalExecutions > 0 {
		stats.AvgLatency = float64(stats.TotalLatency) / float64(stats.TotalExecutions)
		stats.AvgCost = stats.TotalCost / float64(stats.TotalExecutions)
		stats.AvgTokens = float64(stats.TotalTokens) / float64(stats.TotalExecutions)
	}

	return stats
}

// RecorderStats provides statistics about playground usage
type RecorderStats struct {
	TenantID           string            `json:"tenant_id"`
	TotalExecutions    int               `json:"total_executions"`
	StreamedExecutions int               `json:"streamed_executions"`
	ComparisonExecutions int             `json:"comparison_executions"`
	TotalTokens        int64             `json:"total_tokens"`
	TotalCost          float64           `json:"total_cost"`
	TotalLatency       int64             `json:"total_latency"`
	AvgLatency         float64           `json:"avg_latency"`
	AvgCost            float64           `json:"avg_cost"`
	AvgTokens          float64           `json:"avg_tokens"`
	ModelCounts        map[string]int    `json:"model_counts"`
	CreatedAt          time.Time         `json:"created_at"`
}

// sortByCreatedAtDesc sorts histories by creation time descending
func sortByCreatedAtDesc(histories []*PlaygroundHistory) {
	// Simple bubble sort (could use sort.Slice for larger datasets)
	for i := 0; i < len(histories); i++ {
		for j := i + 1; j < len(histories); j++ {
			if histories[i].CreatedAt.Before(histories[j].CreatedAt) {
				histories[i], histories[j] = histories[j], histories[i]
			}
		}
	}
}

// ToJSON converts PlaygroundHistory to JSON string
func (h *PlaygroundHistory) ToJSON() string {
	data, err := json.Marshal(h)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// FromJSON parses PlaygroundHistory from JSON string
func FromJSON(jsonStr string) (*PlaygroundHistory, error) {
	var history PlaygroundHistory
	if err := json.Unmarshal([]byte(jsonStr), &history); err != nil {
		return nil, fmt.Errorf("parse history JSON: %w", err)
	}
	return &history, nil
}