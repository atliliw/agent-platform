// Package rca provides Root Cause Analysis functionality
package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChangeType represents the type of change event
type ChangeType string

const (
	ChangeTypeConfig      ChangeType = "config"
	ChangeTypeDeployment  ChangeType = "deployment"
	ChangeTypeModel       ChangeType = "model"
	ChangeTypeRule        ChangeType = "rule"
	ChangeTypeFeatureFlag ChangeType = "feature_flag"
	ChangeTypeRollback    ChangeType = "rollback"
	ChangeTypeCode        ChangeType = "code"
)

// SeverityLevel represents severity of an issue
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical"
	SeverityHigh     SeverityLevel = "high"
	SeverityMedium   SeverityLevel = "medium"
	SeverityLow      SeverityLevel = "low"
)

// ChangeEvent represents a recorded change
type ChangeEvent struct {
	ID          string      `gorm:"primaryKey"`
	AgentID     string      `gorm:"index"`
	ChangeType  ChangeType  `gorm:"index"`
	ResourceID  string      `gorm:"index"`
	ResourceType string
	Description string
	OldValue    string      // JSON encoded
	NewValue    string      // JSON encoded
	Timestamp   time.Time   `gorm:"index"`
	User        string
	Source      string      // "manual", "auto", "api"
	Metadata    string      // JSON encoded additional metadata
	TenantID    string      `gorm:"index"`
}

// IncidentEvent represents an incident or issue
type IncidentEvent struct {
	ID          string        `gorm:"primaryKey"`
	AgentID     string        `gorm:"index"`
	Title       string
	Description string
	Severity    SeverityLevel `gorm:"index"`
	Impact      string
	DetectedAt  time.Time     `gorm:"index"`
	ResolvedAt  *time.Time
	Status      string        // "active", "resolved", "investigating"
	Metadata    string
	TenantID    string        `gorm:"index"`
}

// AnalysisReport represents the RCA analysis result
type AnalysisReport struct {
	ID             string
	IncidentID     string
	GeneratedAt    time.Time
	SuspectedRootCauses []*RootCause
	RelatedChanges []*ChangeEvent
	Timeline       []*TimelineEvent
	Recommendations []string
	Confidence     float64    // 0-1 score for overall analysis confidence
}

// RootCause represents a suspected root cause
type RootCause struct {
	ChangeEvent   *ChangeEvent
	Correlation   float64    // 0-1 correlation score
	Reason        string
	Evidence      []string
	IsLikely      bool
}

// TimelineEvent represents an event in the timeline
type TimelineEvent struct {
	Timestamp time.Time
	EventType string
	Event     string
	Importance int // 1-5 importance level
}

// Engine is the RCA engine
type Engine struct {
	db           *gorm.DB
	changes      map[string][]*ChangeEvent    // agentID -> changes
	incidents    map[string][]*IncidentEvent  // agentID -> incidents
	reports      map[string]*AnalysisReport   // incidentID -> report
	timeWindow   time.Duration                // Time window to look back for changes
	mu           sync.RWMutex
}

// NewEngine creates a new RCA engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:         db,
		changes:    make(map[string][]*ChangeEvent),
		incidents:  make(map[string][]*IncidentEvent),
		reports:    make(map[string]*AnalysisReport),
		timeWindow: 24 * time.Hour, // Default 24 hour window
	}
	e.loadChanges()
	return e
}

// NewEngineMemory creates an in-memory RCA engine
func NewEngineMemory() *Engine {
	return &Engine{
		changes:    make(map[string][]*ChangeEvent),
		incidents:  make(map[string][]*IncidentEvent),
		reports:    make(map[string]*AnalysisReport),
		timeWindow: 24 * time.Hour,
	}
}

// loadChanges loads change events from database
func (e *Engine) loadChanges() {
	if e.db == nil {
		return
	}

	var changes []ChangeEvent
	if err := e.db.Order("timestamp DESC").Find(&changes).Error; err != nil {
		return
	}

	for _, change := range changes {
		e.changes[change.AgentID] = append(e.changes[change.AgentID], &change)
	}

	var incidents []IncidentEvent
	if err := e.db.Order("detected_at DESC").Find(&incidents).Error; err != nil {
		return
	}

	for _, incident := range incidents {
		e.incidents[incident.AgentID] = append(e.incidents[incident.AgentID], &incident)
	}
}

// RecordChange records a change event
func (e *Engine) RecordChange(ctx context.Context, change *ChangeEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if change.ID == "" {
		change.ID = uuid.New().String()
	}
	change.Timestamp = time.Now()

	if e.db != nil {
		if err := e.db.Create(change).Error; err != nil {
			return fmt.Errorf("record change: %w", err)
		}
	}

	e.changes[change.AgentID] = append([]*ChangeEvent{change}, e.changes[change.AgentID]...)

	return nil
}

// RecordIncident records an incident event
func (e *Engine) RecordIncident(ctx context.Context, incident *IncidentEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if incident.ID == "" {
		incident.ID = uuid.New().String()
	}
	incident.DetectedAt = time.Now()
	incident.Status = "active"

	if e.db != nil {
		if err := e.db.Create(incident).Error; err != nil {
			return fmt.Errorf("record incident: %w", err)
		}
	}

	e.incidents[incident.AgentID] = append([]*IncidentEvent{incident}, e.incidents[incident.AgentID]...)

	return nil
}

// Analyze performs root cause analysis for an incident
func (e *Engine) Analyze(ctx context.Context, incidentID string) (*AnalysisReport, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Find incident
	var incident *IncidentEvent
	for _, incidents := range e.incidents {
		for _, i := range incidents {
			if i.ID == incidentID {
				incident = i
				break
			}
		}
	}

	if incident == nil {
		if e.db != nil {
			incident = &IncidentEvent{}
			if err := e.db.First(incident, "id = ?", incidentID).Error; err != nil {
				return nil, fmt.Errorf("incident not found: %w", err)
			}
		} else {
			return nil, fmt.Errorf("incident not found")
		}
	}

	// Get changes within time window before incident
	windowStart := incident.DetectedAt.Add(-e.timeWindow)
	changes := e.getChangesInWindow(incident.AgentID, windowStart, incident.DetectedAt)

	// Analyze correlations
	rootCauses := e.analyzeCorrelations(incident, changes)

	// Build timeline
	timeline := e.buildTimeline(incident, changes, rootCauses)

	// Generate recommendations
	recommendations := e.generateRecommendations(rootCauses)

	// Calculate confidence score
	confidence := e.calculateConfidence(rootCauses)

	report := &AnalysisReport{
		ID:                  uuid.New().String(),
		IncidentID:          incidentID,
		GeneratedAt:         time.Now(),
		SuspectedRootCauses: rootCauses,
		RelatedChanges:      changes,
		Timeline:            timeline,
		Recommendations:     recommendations,
		Confidence:          confidence,
	}

	e.reports[incidentID] = report

	if e.db != nil {
		// Store report (could be stored as JSON)
		reportJSON, _ := json.Marshal(report)
		// In a real implementation, store this somewhere
		_ = reportJSON
	}

	return report, nil
}

// getChangesInWindow retrieves changes within a time window
func (e *Engine) getChangesInWindow(agentID string, start, end time.Time) []*ChangeEvent {
	var result []*ChangeEvent

	changes := e.changes[agentID]
	for _, change := range changes {
		if change.Timestamp.After(start) && change.Timestamp.Before(end) {
			result = append(result, change)
		}
	}

	// Sort by timestamp descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// analyzeCorrelations analyzes potential correlations between changes and incident
func (e *Engine) analyzeCorrelations(incident *IncidentEvent, changes []*ChangeEvent) []*RootCause {
	var rootCauses []*RootCause

	for _, change := range changes {
		rc := e.calculateChangeCorrelation(incident, change)
		if rc.Correlation > 0.3 { // Threshold for inclusion
			rootCauses = append(rootCauses, rc)
		}
	}

	// Sort by correlation score
	sort.Slice(rootCauses, func(i, j int) bool {
		return rootCauses[i].Correlation > rootCauses[j].Correlation
	})

	return rootCauses
}

// calculateChangeCorrelation calculates correlation score for a change
func (e *Engine) calculateChangeCorrelation(incident *IncidentEvent, change *ChangeEvent) *RootCause {
	rc := &RootCause{
		ChangeEvent: change,
		Correlation: 0,
		IsLikely:    false,
	}

	// Calculate time proximity score (changes closer to incident are more correlated)
	timeDiff := incident.DetectedAt.Sub(change.Timestamp).Minutes()
	timeScore := 1 - math.Min(1, timeDiff/e.timeWindow.Minutes())
	rc.Correlation = timeScore * 0.3 // 30% weight for time proximity

	// Calculate change type relevance
	typeScore := e.getTypeRelevanceScore(incident, change)
	rc.Correlation += typeScore * 0.3 // 30% weight for type relevance

	// Calculate severity alignment
	severityScore := e.getSeverityScore(incident, change)
	rc.Correlation += severityScore * 0.2 // 20% weight for severity

	// Calculate resource relevance
	resourceScore := e.getResourceScore(incident, change)
	rc.Correlation += resourceScore * 0.2 // 20% weight for resource

	// Determine if this is likely root cause
	if rc.Correlation > 0.7 {
		rc.IsLikely = true
	}

	// Generate reason and evidence
	rc.Reason = e.generateReason(incident, change, rc.Correlation)
	rc.Evidence = e.generateEvidence(incident, change)

	return rc
}

// getTypeRelevanceScore calculates relevance based on change type
func (e *Engine) getTypeRelevanceScore(incident *IncidentEvent, change *ChangeEvent) float64 {
	// Certain change types are more likely to cause certain incidents
	switch incident.Impact {
	case "latency":
		if change.ChangeType == ChangeTypeModel || change.ChangeType == ChangeTypeConfig {
			return 0.8
		}
	case "error_rate":
		if change.ChangeType == ChangeTypeCode || change.ChangeType == ChangeTypeDeployment {
			return 0.9
		}
	case "availability":
		if change.ChangeType == ChangeTypeDeployment || change.ChangeType == ChangeTypeRollback {
			return 0.85
		}
	case "cost":
		if change.ChangeType == ChangeTypeModel || change.ChangeType == ChangeTypeConfig {
			return 0.7
		}
	}

	// Default scoring based on change type
	switch change.ChangeType {
	case ChangeTypeDeployment:
		return 0.7
	case ChangeTypeModel:
		return 0.6
	case ChangeTypeConfig:
		return 0.5
	case ChangeTypeFeatureFlag:
		return 0.5
	case ChangeTypeRule:
		return 0.4
	default:
		return 0.3
	}
}

// getSeverityScore calculates score based on severity alignment
func (e *Engine) getSeverityScore(incident *IncidentEvent, change *ChangeEvent) float64 {
	// Critical incidents should correlate with major changes
	switch incident.Severity {
	case SeverityCritical:
		if change.ChangeType == ChangeTypeDeployment || change.ChangeType == ChangeTypeRollback {
			return 0.9
		}
		return 0.5
	case SeverityHigh:
		return 0.6
	case SeverityMedium:
		return 0.4
	case SeverityLow:
		return 0.2
	}
	return 0.3
}

// getResourceScore calculates score based on resource relevance
func (e *Engine) getResourceScore(incident *IncidentEvent, change *ChangeEvent) float64 {
	// If change affects same agent as incident, higher correlation
	if incident.AgentID == change.AgentID {
		return 1.0
	}

	// Check metadata for resource relationships
	var incidentMeta, changeMeta map[string]interface{}
	if incident.Metadata != "" {
		json.Unmarshal([]byte(incident.Metadata), &incidentMeta)
	}
	if change.Metadata != "" {
		json.Unmarshal([]byte(change.Metadata), &changeMeta)
	}

	// Check for related resources
	if incidentMeta != nil && changeMeta != nil {
		if incidentMeta["resource_id"] == changeMeta["resource_id"] {
			return 0.8
		}
	}

	return 0.5
}

// generateReason generates a human-readable reason
func (e *Engine) generateReason(incident *IncidentEvent, change *ChangeEvent, correlation float64) string {
	reasons := []string{
		fmt.Sprintf("Change occurred %.1f minutes before incident detection", incident.DetectedAt.Sub(change.Timestamp).Minutes()),
		fmt.Sprintf("Change type '%s' is commonly associated with '%s' incidents", change.ChangeType, incident.Impact),
	}

	if correlation > 0.7 {
		reasons = append(reasons, "High temporal correlation suggests direct causation")
	} else if correlation > 0.5 {
		reasons = append(reasons, "Moderate correlation suggests potential contribution")
	}

	return strings.Join(reasons, "; ")
}

// generateEvidence generates evidence statements
func (e *Engine) generateEvidence(incident *IncidentEvent, change *ChangeEvent) []string {
	evidence := []string{
		fmt.Sprintf("Change description: %s", change.Description),
		fmt.Sprintf("Changed by: %s", change.User),
		fmt.Sprintf("Change source: %s", change.Source),
	}

	// Parse old/new values for comparison
	if change.OldValue != "" && change.NewValue != "" {
		var oldVal, newVal interface{}
		json.Unmarshal([]byte(change.OldValue), &oldVal)
		json.Unmarshal([]byte(change.NewValue), &newVal)
		evidence = append(evidence, fmt.Sprintf("Value changed from %v to %v", oldVal, newVal))
	}

	return evidence
}

// buildTimeline builds a chronological timeline of events
func (e *Engine) buildTimeline(incident *IncidentEvent, changes []*ChangeEvent, rootCauses []*RootCause) []*TimelineEvent {
	var timeline []*TimelineEvent

	// Add changes
	for _, change := range changes {
		importance := 2
		for _, rc := range rootCauses {
			if rc.ChangeEvent.ID == change.ID && rc.IsLikely {
				importance = 5
			} else if rc.ChangeEvent.ID == change.ID {
				importance = 4
			}
		}

		timeline = append(timeline, &TimelineEvent{
			Timestamp:  change.Timestamp,
			EventType:  "change",
			Event:      fmt.Sprintf("%s: %s", change.ChangeType, change.Description),
			Importance: importance,
		})
	}

	// Add incident detection
	timeline = append(timeline, &TimelineEvent{
		Timestamp:  incident.DetectedAt,
		EventType:  "incident",
		Event:      fmt.Sprintf("Incident detected: %s", incident.Title),
		Importance: 5,
	})

	// Add resolution if resolved
	if incident.ResolvedAt != nil {
		timeline = append(timeline, &TimelineEvent{
			Timestamp:  *incident.ResolvedAt,
			EventType:  "resolution",
			Event:      "Incident resolved",
			Importance: 5,
		})
	}

	// Sort by timestamp
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	return timeline
}

// generateRecommendations generates actionable recommendations
func (e *Engine) generateRecommendations(rootCauses []*RootCause) []string {
	recommendations := []string{}

	// Check for likely root causes
	for _, rc := range rootCauses {
		if rc.IsLikely {
			switch rc.ChangeEvent.ChangeType {
			case ChangeTypeModel:
				recommendations = append(recommendations,
					"Consider rolling back to previous model",
					"Review model configuration changes")
			case ChangeTypeDeployment:
				recommendations = append(recommendations,
					"Consider rolling back deployment",
					"Review deployment logs for errors")
			case ChangeTypeConfig:
				recommendations = append(recommendations,
					"Review configuration changes",
					"Validate configuration values")
			case ChangeTypeFeatureFlag:
				recommendations = append(recommendations,
					"Disable the feature flag temporarily",
					"Review feature flag rollout percentage")
			}
		}
	}

	// Add general recommendations if no specific root cause found
	if len(rootCauses) == 0 || !rootCauses[0].IsLikely {
		recommendations = append(recommendations,
			"Gather more diagnostic data",
			"Review recent system logs",
			"Check external dependencies")
	}

	return recommendations
}

// calculateConfidence calculates overall analysis confidence
func (e *Engine) calculateConfidence(rootCauses []*RootCause) float64 {
	if len(rootCauses) == 0 {
		return 0.1 // Low confidence with no data
	}

	// Use highest correlation as base confidence
	maxCorrelation := rootCauses[0].Correlation

	// Boost confidence if multiple related changes found
	if len(rootCauses) > 3 {
		maxCorrelation += 0.1
	}

	// Boost if likely root cause found
	if rootCauses[0].IsLikely {
		maxCorrelation += 0.15
	}

	return math.Min(1.0, maxCorrelation)
}

// ListChanges lists change events
func (e *Engine) ListChanges(ctx context.Context, agentID string, limit int) ([]*ChangeEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	changes := e.changes[agentID]
	if changes == nil {
		if e.db != nil {
			var c []*ChangeEvent
			query := e.db.Where("agent_id = ?", agentID).Order("timestamp DESC")
			if limit > 0 {
				query = query.Limit(limit)
			}
			if err := query.Find(&c).Error; err != nil {
				return nil, fmt.Errorf("list changes: %w", err)
			}
			return c, nil
		}
		return nil, nil
	}

	if limit > 0 && len(changes) > limit {
		return changes[:limit], nil
	}
	return changes, nil
}

// ListIncidents lists incident events
func (e *Engine) ListIncidents(ctx context.Context, agentID, status string, limit int) ([]*IncidentEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var incidents []*IncidentEvent
	for _, i := range e.incidents[agentID] {
		if status == "" || i.Status == status {
			incidents = append(incidents, i)
		}
	}

	if incidents == nil && e.db != nil {
		var i []*IncidentEvent
		query := e.db.Where("agent_id = ?", agentID)
		if status != "" {
			query = query.Where("status = ?", status)
		}
		query = query.Order("detected_at DESC")
		if limit > 0 {
			query = query.Limit(limit)
		}
		if err := query.Find(&i).Error; err != nil {
			return nil, fmt.Errorf("list incidents: %w", err)
		}
		return i, nil
	}

	if limit > 0 && len(incidents) > limit {
		return incidents[:limit], nil
	}
	return incidents, nil
}

// ListReports lists analysis reports
func (e *Engine) ListReports(ctx context.Context, agentID string, limit int) ([]*AnalysisReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var reports []*AnalysisReport

	// Get incidents for agent
	incidents := e.incidents[agentID]
	for _, incident := range incidents {
		if report, exists := e.reports[incident.ID]; exists {
			reports = append(reports, report)
		}
	}

	if limit > 0 && len(reports) > limit {
		return reports[:limit], nil
	}
	return reports, nil
}

// ResolveIncident marks an incident as resolved
func (e *Engine) ResolveIncident(ctx context.Context, incidentID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	for _, incidents := range e.incidents {
		for _, i := range incidents {
			if i.ID == incidentID {
				i.Status = "resolved"
				i.ResolvedAt = &now

				if e.db != nil {
					e.db.Model(i).Updates(map[string]interface{}{
						"status":     "resolved",
						"resolved_at": now,
					})
				}
				return nil
			}
		}
	}

	return fmt.Errorf("incident not found")
}

// SetTimeWindow sets the analysis time window
func (e *Engine) SetTimeWindow(window time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.timeWindow = window
}

// DeleteChange deletes a change event
func (e *Engine) DeleteChange(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Delete(&ChangeEvent{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete change: %w", err)
		}
	}

	for agentID, changes := range e.changes {
		for i, c := range changes {
			if c.ID == id {
				e.changes[agentID] = append(changes[:i], changes[i+1:]...)
				return nil
			}
		}
	}

	return nil
}
