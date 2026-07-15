// Package prompt provides performance tracking for prompt versions
package prompt

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PerformanceTracker tracks and analyzes prompt performance metrics
type PerformanceTracker struct {
	db       *gorm.DB
	records  map[string][]UsageRecord // versionID -> records (in-memory cache)
	mu       sync.RWMutex
	maxCache int // Maximum records to keep per version in memory
}

// NewPerformanceTracker creates a new performance tracker
func NewPerformanceTracker(db *gorm.DB) *PerformanceTracker {
	return &PerformanceTracker{
		db:       db,
		records:  make(map[string][]UsageRecord),
		maxCache: 1000,
	}
}

// NewPerformanceTrackerMemory creates an in-memory performance tracker
func NewPerformanceTrackerMemory() *PerformanceTracker {
	return &PerformanceTracker{
		records:  make(map[string][]UsageRecord),
		maxCache: 1000,
	}
}

// RecordUsage records a single usage event
func (t *PerformanceTracker) RecordUsage(ctx context.Context, versionID, sessionID string, success bool, latencyMs int64, inputTokens, outputTokens int64, cost float64, userRating float64, metadata map[string]interface{}) error {
	record := UsageRecord{
		ID:           uuid.New().String(),
		VersionID:    versionID,
		SessionID:    sessionID,
		Success:      success,
		LatencyMs:    latencyMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		UserRating:   userRating,
		Timestamp:    time.Now(),
	}

	if metadata != nil {
		metaBytes, _ := json.Marshal(metadata)
		record.Metadata = string(metaBytes)
	}

	// Save to database
	if t.db != nil {
		if err := t.db.Create(&record).Error; err != nil {
			return fmt.Errorf("save usage record: %w", err)
		}
	}

	// Update in-memory cache
	t.mu.Lock()
	defer t.mu.Unlock()

	t.records[versionID] = append(t.records[versionID], record)
	if len(t.records[versionID]) > t.maxCache {
		t.records[versionID] = t.records[versionID][1:]
	}

	return nil
}

// GetPerformance calculates aggregated performance metrics for a version
func (t *PerformanceTracker) GetPerformance(ctx context.Context, versionID string, periodStart, periodEnd time.Time) (*PromptPerformance, error) {
	if periodEnd.IsZero() {
		periodEnd = time.Now()
	}
	if periodStart.IsZero() {
		periodStart = periodEnd.AddDate(0, 0, -7) // Default to last 7 days
	}

	// Query from database
	if t.db != nil {
		var records []UsageRecord
		if err := t.db.Where("version_id = ? AND timestamp >= ? AND timestamp <= ?", versionID, periodStart, periodEnd).
			Find(&records).Error; err != nil {
			return nil, fmt.Errorf("query usage records: %w", err)
		}
		return t.calculatePerformance(versionID, records, periodStart, periodEnd), nil
	}

	// Use in-memory cache
	t.mu.RLock()
	records := t.filterByPeriod(t.records[versionID], periodStart, periodEnd)
	t.mu.RUnlock()

	return t.calculatePerformance(versionID, records, periodStart, periodEnd), nil
}

// GetPerformanceTrend analyzes performance trends over time
func (t *PerformanceTracker) GetPerformanceTrend(ctx context.Context, versionID string, days int) (*PerformanceTrend, error) {
	if days <= 0 {
		days = 30
	}

	now := time.Now()
	var dataPoints []PerformanceDataPoint

	// Calculate daily metrics
	for i := days - 1; i >= 0; i-- {
		dayStart := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		dayEnd := dayStart.Add(24 * time.Hour)

		var records []UsageRecord
		if t.db != nil {
			t.db.Where("version_id = ? AND timestamp >= ? AND timestamp < ?", versionID, dayStart, dayEnd).Find(&records)
		} else {
			t.mu.RLock()
			records = t.filterByPeriod(t.records[versionID], dayStart, dayEnd)
			t.mu.RUnlock()
		}

		perf := t.calculatePerformance(versionID, records, dayStart, dayEnd)
		dataPoints = append(dataPoints, PerformanceDataPoint{
			Timestamp:   dayStart,
			SuccessRate: perf.SuccessRate,
			AvgLatency:  perf.AvgLatency,
			AvgCost:     perf.AvgCost,
			UserRating:  perf.UserRating,
			CallCount:   perf.TotalCalls,
		})
	}

	// Determine trend
	trend := "stable"
	changeRate := 0.0
	if len(dataPoints) >= 7 {
		// Compare last 7 days to previous 7 days
		recent := dataPoints[len(dataPoints)-7:]
		previous := dataPoints[len(dataPoints)-14 : len(dataPoints)-7]

		recentSuccessRate := avgSuccessRate(recent)
		previousSuccessRate := avgSuccessRate(previous)

		if previousSuccessRate > 0 {
			changeRate = (recentSuccessRate - previousSuccessRate) / previousSuccessRate
			if changeRate > 0.05 {
				trend = "improving"
			} else if changeRate < -0.05 {
				trend = "declining"
			}
		}
	}

	return &PerformanceTrend{
		VersionID:  versionID,
		DataPoints: dataPoints,
		Trend:      trend,
		ChangeRate: changeRate,
	}, nil
}

// ComparePerformance compares performance between two versions
func (t *PerformanceTracker) ComparePerformance(ctx context.Context, version1ID, version2ID string, periodStart, periodEnd time.Time) (map[string]interface{}, error) {
	perf1, err := t.GetPerformance(ctx, version1ID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	perf2, err := t.GetPerformance(ctx, version2ID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"version1": perf1,
		"version2": perf2,
		"delta": map[string]interface{}{
			"success_rate": perf2.SuccessRate - perf1.SuccessRate,
			"latency":      perf2.AvgLatency - perf1.AvgLatency,
			"cost":         perf2.AvgCost - perf1.AvgCost,
			"user_rating":  perf2.UserRating - perf1.UserRating,
		},
	}, nil
}

// GetTopPerformers returns the top performing versions
func (t *PerformanceTracker) GetTopPerformers(ctx context.Context, promptID string, limit int, metric string) ([]*PromptPerformance, error) {
	if limit <= 0 {
		limit = 10
	}

	// Get all versions for this prompt
	var versions []PromptVersion
	if t.db != nil {
		if err := t.db.Where("prompt_id = ?", promptID).Find(&versions).Error; err != nil {
			return nil, fmt.Errorf("query versions: %w", err)
		}
	} else {
		return nil, fmt.Errorf("in-memory mode does not support top performers query")
	}

	// Calculate performance for each version
	var performances []*PromptPerformance
	periodEnd := time.Now()
	periodStart := periodEnd.AddDate(0, 0, -7)

	for _, v := range versions {
		perf, err := t.GetPerformance(ctx, v.ID, periodStart, periodEnd)
		if err != nil {
			continue
		}
		performances = append(performances, perf)
	}

	// Sort by metric
	sortPerformancesByMetric(performances, metric)

	if len(performances) > limit {
		performances = performances[:limit]
	}

	return performances, nil
}

// RecordFeedback records user feedback for a version
func (t *PerformanceTracker) RecordFeedback(ctx context.Context, versionID, sessionID string, rating float64, comment string) error {
	// Find existing record for this session
	var record UsageRecord
	if t.db != nil {
		if err := t.db.Where("version_id = ? AND session_id = ?", versionID, sessionID).First(&record).Error; err != nil {
			return fmt.Errorf("find usage record: %w", err)
		}

		record.UserRating = rating
		if comment != "" {
			var metadata map[string]interface{}
			json.Unmarshal([]byte(record.Metadata), &metadata)
			if metadata == nil {
				metadata = make(map[string]interface{})
			}
			metadata["comment"] = comment
			metaBytes, _ := json.Marshal(metadata)
			record.Metadata = string(metaBytes)
		}

		if err := t.db.Save(&record).Error; err != nil {
			return fmt.Errorf("update usage record: %w", err)
		}
	}

	// Update in-memory cache
	t.mu.Lock()
	defer t.mu.Unlock()

	records := t.records[versionID]
	for i, r := range records {
		if r.SessionID == sessionID {
			records[i].UserRating = rating
			break
		}
	}

	return nil
}

// Helper functions

func (t *PerformanceTracker) calculatePerformance(versionID string, records []UsageRecord, periodStart, periodEnd time.Time) *PromptPerformance {
	if len(records) == 0 {
		return &PromptPerformance{
			ID:          uuid.New().String(),
			VersionID:   versionID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
		}
	}

	var totalCalls, successCalls int64
	var totalLatency, totalCost, totalRating float64
	var totalInputTokens, totalOutputTokens int64
	var ratingCount int64

	for _, r := range records {
		totalCalls++
		if r.Success {
			successCalls++
		}
		totalLatency += float64(r.LatencyMs)
		totalInputTokens += r.InputTokens
		totalOutputTokens += r.OutputTokens
		totalCost += r.Cost
		if r.UserRating > 0 {
			totalRating += r.UserRating
			ratingCount++
		}
	}

	successRate := float64(0)
	if totalCalls > 0 {
		successRate = float64(successCalls) / float64(totalCalls)
	}

	avgLatency := float64(0)
	if totalCalls > 0 {
		avgLatency = totalLatency / float64(totalCalls)
	}

	avgCost := float64(0)
	if totalCalls > 0 {
		avgCost = totalCost / float64(totalCalls)
	}

	avgRating := float64(0)
	if ratingCount > 0 {
		avgRating = totalRating / float64(ratingCount)
	}

	return &PromptPerformance{
		ID:              uuid.New().String(),
		VersionID:       versionID,
		TotalCalls:      totalCalls,
		SuccessCalls:    successCalls,
		SuccessRate:     successRate,
		AvgLatency:      avgLatency,
		AvgInputTokens:  totalInputTokens / totalCalls,
		AvgOutputTokens: totalOutputTokens / totalCalls,
		AvgTotalTokens:  (totalInputTokens + totalOutputTokens) / totalCalls,
		AvgCost:         avgCost,
		UserRating:      avgRating,
		FeedbackCount:   ratingCount,
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
	}
}

func (t *PerformanceTracker) filterByPeriod(records []UsageRecord, start, end time.Time) []UsageRecord {
	var filtered []UsageRecord
	for _, r := range records {
		if r.Timestamp.After(start) && r.Timestamp.Before(end) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func avgSuccessRate(points []PerformanceDataPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	var total float64
	for _, p := range points {
		total += p.SuccessRate
	}
	return total / float64(len(points))
}

func sortPerformancesByMetric(perfs []*PromptPerformance, metric string) {
	// Simple bubble sort for now
	for i := 0; i < len(perfs)-1; i++ {
		for j := i + 1; j < len(perfs); j++ {
			var swap bool
			switch metric {
			case "success_rate":
				swap = perfs[j].SuccessRate > perfs[i].SuccessRate
			case "latency":
				swap = perfs[j].AvgLatency < perfs[i].AvgLatency // Lower is better
			case "cost":
				swap = perfs[j].AvgCost < perfs[i].AvgCost // Lower is better
			case "rating":
				swap = perfs[j].UserRating > perfs[i].UserRating
			default:
				swap = perfs[j].SuccessRate > perfs[i].SuccessRate
			}
			if swap {
				perfs[i], perfs[j] = perfs[j], perfs[i]
			}
		}
	}
}
