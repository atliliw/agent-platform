// Package slo provides SLO (Service Level Objective) management
package slo

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WindowType defines the time window for SLO calculation
type WindowType string

const (
	WindowRolling1h   WindowType = "rolling-1h"
	WindowRolling6h   WindowType = "rolling-6h"
	WindowRolling24h  WindowType = "rolling-24h"
	WindowRolling7d   WindowType = "rolling-7d"
	WindowRolling30d  WindowType = "rolling-30d"
	WindowCalendarDay WindowType = "calendar-day"
	WindowCalendarWeek WindowType = "calendar-week"
	WindowCalendarMonth WindowType = "calendar-month"
)

// SLOType defines the type of SLO
type SLOType string

const (
	SLOTypeLatency      SLOType = "latency"
	SLOTypeSuccessRate  SLOType = "success_rate"
	SLOTypeAvailability SLOType = "availability"
	SLOTypeErrorBudget  SLOType = "error_budget"
)

// SLOStatus represents the health status
type SLOStatus string

const (
	StatusHealthy   SLOStatus = "healthy"
	StatusWarning   SLOStatus = "warning"
	StatusCritical  SLOStatus = "critical"
	StatusBreaching SLOStatus = "breaching"
)

// AlertSeverity defines alert severity levels
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// SLODefinition represents an SLO configuration
type SLODefinition struct {
	ID              string      `gorm:"primaryKey"`
	AgentID         string      `gorm:"index"`
	Name            string
	Type            SLOType
	Target          float64     // Target value (e.g., 0.999 for 99.9%)
	Window          WindowType  // Time window type
	AlertThreshold  float64     // Alert when burn rate exceeds this
	BurnRateAlert   bool        // Enable burn rate alerting
	TenantID        string      `gorm:"index"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SLOEvent represents a recorded event for SLO calculation
type SLOEvent struct {
	ID          string    `gorm:"primaryKey"`
	SLOID       string    `gorm:"index"`
	AgentID     string    `gorm:"index"`
	EventType   string    // "success", "failure", "latency"
	Value       float64   // Latency in ms, or 1/0 for success/failure
	Labels      string    // JSON labels
	Timestamp   time.Time `gorm:"index"`
}

// BurnRateAlert represents a burn rate alert
type BurnRateAlert struct {
	SLOID       string
	Name        string
	BurnRate    float64
	Threshold   float64
	Severity    AlertSeverity
	Status      SLOStatus
	Timestamp   time.Time
}

// AlertCallback is called when an alert is triggered
type AlertCallback func(alert BurnRateAlert)

// SLOStatusResult represents the current status of an SLO
type SLOStatusResult struct {
	SLOID            string
	Name             string
	Type             SLOType
	Target           float64
	Current          float64
	ErrorBudget      float64    // Remaining error budget (0-1)
	ErrorBudgetUsed  float64    // Error budget used (0-1)
	BurnRate         float64    // Current burn rate
	Status           SLOStatus
	Window           WindowType
	TotalEvents      int64
	GoodEvents       int64
	BadEvents        int64
	LastUpdated      time.Time
}

// Manager is the SLO manager
type Manager struct {
	db           *gorm.DB
	definitions  map[string]*SLODefinition
	statuses     map[string]*sloStatusInternal
	eventBuffers map[string]*eventBuffer
	alertCB      AlertCallback
	mu           sync.RWMutex
}

type sloStatusInternal struct {
	current        float64
	errorBudget    float64
	burnRate       float64
	status         SLOStatus
	totalEvents    int64
	goodEvents     int64
	badEvents      int64
	lastUpdated    time.Time
}

type eventBuffer struct {
	events []timedEvent
	mu     sync.RWMutex
}

type timedEvent struct {
	timestamp time.Time
	value     float64
	isGood    bool
	latency   float64
}

// NewManager creates a new SLO manager
func NewManager(db *gorm.DB) *Manager {
	m := &Manager{
		db:           db,
		definitions:  make(map[string]*SLODefinition),
		statuses:     make(map[string]*sloStatusInternal),
		eventBuffers: make(map[string]*eventBuffer),
	}
	m.loadDefinitions()
	return m
}

// NewManagerMemory creates an in-memory SLO manager
func NewManagerMemory() *Manager {
	return &Manager{
		definitions:  make(map[string]*SLODefinition),
		statuses:     make(map[string]*sloStatusInternal),
		eventBuffers: make(map[string]*eventBuffer),
	}
}

// loadDefinitions loads SLO definitions from database
func (m *Manager) loadDefinitions() {
	if m.db == nil {
		return
	}

	var defs []SLODefinition
	if err := m.db.Find(&defs).Error; err != nil {
		return
	}

	for _, def := range defs {
		m.definitions[def.ID] = &def
		m.statuses[def.ID] = &sloStatusInternal{
			status:      StatusHealthy,
			errorBudget: 1.0,
			lastUpdated: time.Now(),
		}
		m.eventBuffers[def.ID] = &eventBuffer{
			events: make([]timedEvent, 0, 10000),
		}
	}
}

// SetAlertCallback sets the alert callback function
func (m *Manager) SetAlertCallback(cb AlertCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertCB = cb
}

// CreateSLO creates a new SLO definition
func (m *Manager) CreateSLO(ctx context.Context, def *SLODefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	def.CreatedAt = time.Now()
	def.UpdatedAt = time.Now()

	if def.Window == "" {
		def.Window = WindowRolling24h
	}
	if def.AlertThreshold == 0 {
		def.AlertThreshold = 0.02 // Default 2% burn rate threshold
	}

	if m.db != nil {
		if err := m.db.Create(def).Error; err != nil {
			return fmt.Errorf("create slo: %w", err)
		}
	}

	m.definitions[def.ID] = def
	m.statuses[def.ID] = &sloStatusInternal{
		status:      StatusHealthy,
		errorBudget: 1.0,
		lastUpdated: time.Now(),
	}
	m.eventBuffers[def.ID] = &eventBuffer{
		events: make([]timedEvent, 0, 10000),
	}

	return nil
}

// GetSLO retrieves an SLO definition
func (m *Manager) GetSLO(ctx context.Context, id string) (*SLODefinition, error) {
	m.mu.RLock()
	def, exists := m.definitions[id]
	m.mu.RUnlock()

	if exists {
		return def, nil
	}

	if m.db != nil {
		var d SLODefinition
		if err := m.db.First(&d, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get slo: %w", err)
		}
		return &d, nil
	}

	return nil, fmt.Errorf("slo not found")
}

// ListSLOs lists all SLO definitions
func (m *Manager) ListSLOs(ctx context.Context, agentID, tenantID string) ([]*SLODefinition, error) {
	if m.db != nil {
		query := m.db.Model(&SLODefinition{})
		if agentID != "" {
			query = query.Where("agent_id = ?", agentID)
		}
		if tenantID != "" {
			query = query.Where("tenant_id = ?", tenantID)
		}

		var defs []*SLODefinition
		if err := query.Find(&defs).Error; err != nil {
			return nil, fmt.Errorf("list slos: %w", err)
		}
		return defs, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SLODefinition
	for _, def := range m.definitions {
		if agentID != "" && def.AgentID != agentID {
			continue
		}
		if tenantID != "" && def.TenantID != tenantID {
			continue
		}
		result = append(result, def)
	}
	return result, nil
}

// UpdateSLO updates an SLO definition
func (m *Manager) UpdateSLO(ctx context.Context, def *SLODefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	def.UpdatedAt = time.Now()

	if m.db != nil {
		if err := m.db.Save(def).Error; err != nil {
			return fmt.Errorf("update slo: %w", err)
		}
	}

	m.definitions[def.ID] = def
	return nil
}

// DeleteSLO deletes an SLO definition
func (m *Manager) DeleteSLO(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db != nil {
		m.db.Where("slo_id = ?", id).Delete(&SLOEvent{})
		if err := m.db.Delete(&SLODefinition{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete slo: %w", err)
		}
	}

	delete(m.definitions, id)
	delete(m.statuses, id)
	delete(m.eventBuffers, id)
	return nil
}

// RecordEvent records an event for SLO calculation
func (m *Manager) RecordEvent(ctx context.Context, sloID string, success bool, latencyMs float64) error {
	m.mu.RLock()
	_, exists := m.definitions[sloID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("slo not found: %s", sloID)
	}

	// Add to buffer
	if buf, ok := m.eventBuffers[sloID]; ok {
		buf.mu.Lock()
		buf.events = append(buf.events, timedEvent{
			timestamp: time.Now(),
			isGood:    success,
			latency:   latencyMs,
		})
		buf.mu.Unlock()
	}

	// Persist to database
	if m.db != nil {
		event := &SLOEvent{
			ID:        uuid.New().String(),
			SLOID:     sloID,
			EventType: "request",
			Value:     latencyMs,
			Timestamp: time.Now(),
		}
		if !success {
			event.EventType = "failure"
		}
		if err := m.db.Create(event).Error; err != nil {
			return fmt.Errorf("record event: %w", err)
		}
	}

	// Recalculate status
	m.recalculateStatus(sloID)

	return nil
}

// RecordSuccess records a successful event
func (m *Manager) RecordSuccess(ctx context.Context, sloID string, latencyMs float64) error {
	return m.RecordEvent(ctx, sloID, true, latencyMs)
}

// RecordFailure records a failed event
func (m *Manager) RecordFailure(ctx context.Context, sloID string) error {
	return m.RecordEvent(ctx, sloID, false, 0)
}

// recalculateStatus recalculates SLO status
func (m *Manager) recalculateStatus(sloID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	def, exists := m.definitions[sloID]
	if !exists {
		return
	}

	status := m.statuses[sloID]
	if status == nil {
		return
	}

	// Get events from buffer
	buf := m.eventBuffers[sloID]
	if buf == nil {
		return
	}

	buf.mu.RLock()
	events := buf.events
	buf.mu.RUnlock()

	// Filter events by window
	windowDuration := getWindowDuration(def.Window)
	cutoff := time.Now().Add(-windowDuration)

	var windowEvents []timedEvent
	for _, e := range events {
		if e.timestamp.After(cutoff) {
			windowEvents = append(windowEvents, e)
		}
	}

	// Calculate metrics
	status.totalEvents = int64(len(windowEvents))
	status.goodEvents = 0
	status.badEvents = 0

	var totalLatency float64
	var latencyCount int64

	for _, e := range windowEvents {
		if e.isGood {
			status.goodEvents++
		} else {
			status.badEvents++
		}
		if e.latency > 0 {
			totalLatency += e.latency
			latencyCount++
		}
	}

	// Calculate current value based on SLO type
	switch def.Type {
	case SLOTypeSuccessRate, SLOTypeAvailability:
		if status.totalEvents > 0 {
			status.current = float64(status.goodEvents) / float64(status.totalEvents)
		}
	case SLOTypeLatency:
		if latencyCount > 0 {
			status.current = totalLatency / float64(latencyCount)
		}
	}

	// Calculate error budget
	// Error budget = (current - target) / (1 - target)
	if def.Target < 1 && def.Type != SLOTypeLatency {
		if status.current > 0 {
			status.errorBudget = 1 - (status.current-def.Target)/(1-def.Target)
			if status.errorBudget < 0 {
				status.errorBudget = 0
			}
		}
	} else if def.Type == SLOTypeLatency {
		// For latency, budget decreases as latency increases
		if status.current > 0 {
			status.errorBudget = 1 - status.current/def.Target
			if status.errorBudget < 0 {
				status.errorBudget = 0
			}
		}
	}

	// Calculate burn rate
	// Burn rate = (1 - current) / window_duration_in_hours
	if len(windowEvents) >= 2 {
		timeSpan := windowEvents[len(windowEvents)-1].timestamp.Sub(windowEvents[0].timestamp).Hours()
		if timeSpan > 0 {
			status.burnRate = (1 - status.current) / timeSpan
		}
	}

	// Determine status
	if status.errorBudget <= 0 {
		status.status = StatusBreaching
	} else if status.errorBudget < 0.1 {
		status.status = StatusCritical
	} else if status.errorBudget < 0.3 {
		status.status = StatusWarning
	} else {
		status.status = StatusHealthy
	}

	status.lastUpdated = time.Now()

	// Check burn rate alert
	if def.BurnRateAlert && status.burnRate > def.AlertThreshold {
		m.triggerAlert(BurnRateAlert{
			SLOID:     sloID,
			Name:      def.Name,
			BurnRate:  status.burnRate,
			Threshold: def.AlertThreshold,
			Severity:  AlertSeverityWarning,
			Status:    status.status,
			Timestamp: time.Now(),
		})
	}
}

// GetStatus returns the current status of an SLO
func (m *Manager) GetStatus(ctx context.Context, sloID string) (*SLOStatusResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	def, exists := m.definitions[sloID]
	if !exists {
		return nil, fmt.Errorf("slo not found")
	}

	status := m.statuses[sloID]
	if status == nil {
		return nil, fmt.Errorf("slo status not found")
	}

	return &SLOStatusResult{
		SLOID:           sloID,
		Name:            def.Name,
		Type:            def.Type,
		Target:          def.Target,
		Current:         status.current,
		ErrorBudget:     status.errorBudget,
		ErrorBudgetUsed: 1 - status.errorBudget,
		BurnRate:        status.burnRate,
		Status:          status.status,
		Window:          def.Window,
		TotalEvents:     status.totalEvents,
		GoodEvents:      status.goodEvents,
		BadEvents:       status.badEvents,
		LastUpdated:     status.lastUpdated,
	}, nil
}

// GetErrorBudget returns the remaining error budget
func (m *Manager) GetErrorBudget(ctx context.Context, sloID string) (float64, error) {
	status, err := m.GetStatus(ctx, sloID)
	if err != nil {
		return 0, err
	}
	return status.ErrorBudget, nil
}

// CheckBurnRate checks if burn rate exceeds threshold
func (m *Manager) CheckBurnRate(ctx context.Context, sloID string) (bool, float64, error) {
	status, err := m.GetStatus(ctx, sloID)
	if err != nil {
		return false, 0, err
	}

	def, err := m.GetSLO(ctx, sloID)
	if err != nil {
		return false, 0, err
	}

	return status.BurnRate > def.AlertThreshold, status.BurnRate, nil
}

// EvaluateAll evaluates all SLOs and returns their statuses
func (m *Manager) EvaluateAll(ctx context.Context, agentID string) ([]*SLOStatusResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SLOStatusResult

	for id, def := range m.definitions {
		if agentID != "" && def.AgentID != agentID {
			continue
		}

		status := m.statuses[id]
		if status == nil {
			continue
		}

		results = append(results, &SLOStatusResult{
			SLOID:           id,
			Name:            def.Name,
			Type:            def.Type,
			Target:          def.Target,
			Current:         status.current,
			ErrorBudget:     status.errorBudget,
			ErrorBudgetUsed: 1 - status.errorBudget,
			BurnRate:        status.burnRate,
			Status:          status.status,
			Window:          def.Window,
			TotalEvents:     status.totalEvents,
			GoodEvents:      status.goodEvents,
			BadEvents:       status.badEvents,
			LastUpdated:     status.lastUpdated,
		})
	}

	return results, nil
}

// triggerAlert triggers an alert callback
func (m *Manager) triggerAlert(alert BurnRateAlert) {
	if m.alertCB != nil {
		go m.alertCB(alert)
	}
}

// getWindowDuration returns the duration for a window type
func getWindowDuration(window WindowType) time.Duration {
	switch window {
	case WindowRolling1h:
		return time.Hour
	case WindowRolling6h:
		return 6 * time.Hour
	case WindowRolling24h:
		return 24 * time.Hour
	case WindowRolling7d:
		return 7 * 24 * time.Hour
	case WindowRolling30d:
		return 30 * 24 * time.Hour
	case WindowCalendarDay:
		return 24 * time.Hour
	case WindowCalendarWeek:
		return 7 * 24 * time.Hour
	case WindowCalendarMonth:
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// CleanupOldEvents removes events older than the maximum window
func (m *Manager) CleanupOldEvents(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	maxWindow := 30 * 24 * time.Hour // Keep 30 days max
	cutoff := time.Now().Add(-maxWindow)

	for sloID, buf := range m.eventBuffers {
		buf.mu.Lock()
		var newEvents []timedEvent
		for _, e := range buf.events {
			if e.timestamp.After(cutoff) {
				newEvents = append(newEvents, e)
			}
		}
		buf.events = newEvents
		buf.mu.Unlock()

		// Also cleanup database
		if m.db != nil {
			m.db.Where("slo_id = ? AND timestamp < ?", sloID, cutoff).Delete(&SLOEvent{})
		}
	}

	return nil
}

// GetAlertStatus returns alerts for all SLOs
func (m *Manager) GetAlertStatus() map[string]SLOStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make(map[string]SLOStatus)
	for id, status := range m.statuses {
		if status.status != StatusHealthy {
			if def, ok := m.definitions[id]; ok {
				alerts[def.Name] = status.status
			}
		}
	}
	return alerts
}

// CalculatePercentile calculates a percentile value from latency events
func (m *Manager) CalculatePercentile(ctx context.Context, sloID string, percentile float64) (float64, error) {
	buf, exists := m.eventBuffers[sloID]
	if !exists {
		return 0, fmt.Errorf("slo not found")
	}

	buf.mu.RLock()
	defer buf.mu.RUnlock()

	var latencies []float64
	for _, e := range buf.events {
		if e.latency > 0 {
			latencies = append(latencies, e.latency)
		}
	}

	if len(latencies) == 0 {
		return 0, nil
	}

	// Sort latencies (simple insertion sort for small arrays)
	for i := 1; i < len(latencies); i++ {
		for j := i; j > 0 && latencies[j] < latencies[j-1]; j-- {
			latencies[j], latencies[j-1] = latencies[j-1], latencies[j]
		}
	}

	index := int(math.Ceil(float64(len(latencies))*percentile/100)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(latencies) {
		index = len(latencies) - 1
	}

	return latencies[index], nil
}
