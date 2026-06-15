// Package rollback provides configuration rollback functionality
package rollback

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RollbackConfig represents a rollback configuration
type RollbackConfig struct {
	ID              string    `gorm:"primaryKey"`
	AgentID         string    `gorm:"index"`
	Name            string
	ConfigType      string    // "agent_config", "model_config", "rule_config", "feature_flag"
	TargetID        string    // ID of the target configuration
	MaxSnapshots    int       // Maximum number of snapshots to keep
	CoolDownPeriod  int       // Minutes between rollback attempts
	AutoRollback    bool      // Enable automatic rollback on failure
	RollbackOnSLO   bool      // Rollback when SLO threshold is breached
	SLOThreshold    float64   // SLO threshold for auto rollback
	TenantID        string    `gorm:"index"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ConfigSnapshot represents a configuration snapshot
type ConfigSnapshot struct {
	ID           string    `gorm:"primaryKey"`
	ConfigID     string    `gorm:"index"`
	SnapshotData string    // JSON encoded configuration
	Version      string
	Description  string
	CreatedAt    time.Time
	CreatedBy    string
	IsActive     bool      `gorm:"index"`
}

// RollbackEvent represents a rollback execution
type RollbackEvent struct {
	ID            string    `gorm:"primaryKey"`
	ConfigID      string    `gorm:"index"`
	SnapshotID    string    `gorm:"index"`
	EventType     string    // "rollback", "restore", "auto_rollback"
	TriggeredBy   string    // "manual", "auto", "slo_breach"
	FromVersion   string
	ToVersion     string
	Success       bool
	Error         string
	DurationMs    int64
	Timestamp     time.Time
}

// RollbackStatus represents current rollback status
type RollbackStatus struct {
	ConfigID      string
	CurrentVersion string
	LastSnapshot   time.Time
	CanRollback    bool
	LastRollback   time.Time
	RollbackCount  int64
	NextRollback   time.Time // When cooldown expires
}

// Engine is the rollback engine
type Engine struct {
	db      *gorm.DB
	configs map[string]*RollbackConfig
	snapshots map[string][]*ConfigSnapshot // configID -> snapshots
	events  map[string][]*RollbackEvent    // configID -> events
	mu      sync.RWMutex
}

// NewEngine creates a new rollback engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:        db,
		configs:   make(map[string]*RollbackConfig),
		snapshots: make(map[string][]*ConfigSnapshot),
		events:    make(map[string][]*RollbackEvent),
	}
	e.loadConfigs()
	return e
}

// NewEngineMemory creates an in-memory rollback engine
func NewEngineMemory() *Engine {
	return &Engine{
		configs:   make(map[string]*RollbackConfig),
		snapshots: make(map[string][]*ConfigSnapshot),
		events:    make(map[string][]*RollbackEvent),
	}
}

// loadConfigs loads rollback configurations from database
func (e *Engine) loadConfigs() {
	if e.db == nil {
		return
	}

	var configs []RollbackConfig
	if err := e.db.Find(&configs).Error; err != nil {
		return
	}

	for _, config := range configs {
		e.configs[config.ID] = &config

		// Load snapshots
		var snaps []ConfigSnapshot
		e.db.Where("config_id = ?", config.ID).Order("created_at DESC").Find(&snaps)
		for _, snap := range snaps {
			e.snapshots[config.ID] = append(e.snapshots[config.ID], &snap)
		}

		// Load events
		var evts []RollbackEvent
		e.db.Where("config_id = ?", config.ID).Order("timestamp DESC").Find(&evts)
		for _, evt := range evts {
			e.events[config.ID] = append(e.events[config.ID], &evt)
		}
	}
}

// CreateConfig creates a rollback configuration
func (e *Engine) CreateConfig(ctx context.Context, config *RollbackConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	if config.MaxSnapshots == 0 {
		config.MaxSnapshots = 10
	}
	if config.CoolDownPeriod == 0 {
		config.CoolDownPeriod = 5 // 5 minutes default
	}

	if e.db != nil {
		if err := e.db.Create(config).Error; err != nil {
			return fmt.Errorf("create rollback config: %w", err)
		}
	}

	e.configs[config.ID] = config
	e.snapshots[config.ID] = make([]*ConfigSnapshot, 0)
	e.events[config.ID] = make([]*RollbackEvent, 0)

	return nil
}

// GetConfig retrieves a rollback configuration
func (e *Engine) GetConfig(ctx context.Context, id string) (*RollbackConfig, error) {
	e.mu.RLock()
	config, exists := e.configs[id]
	e.mu.RUnlock()

	if exists {
		return config, nil
	}

	if e.db != nil {
		var c RollbackConfig
		if err := e.db.First(&c, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get rollback config: %w", err)
		}
		return &c, nil
	}

	return nil, fmt.Errorf("rollback config not found")
}

// UpdateConfig updates a rollback configuration
func (e *Engine) UpdateConfig(ctx context.Context, config *RollbackConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	config.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Save(config).Error; err != nil {
			return fmt.Errorf("update rollback config: %w", err)
		}
	}

	e.configs[config.ID] = config
	return nil
}

// ListConfigs lists rollback configurations
func (e *Engine) ListConfigs(ctx context.Context, agentID, tenantID string) ([]*RollbackConfig, error) {
	if e.db != nil {
		query := e.db.Model(&RollbackConfig{})
		if agentID != "" {
			query = query.Where("agent_id = ?", agentID)
		}
		if tenantID != "" {
			query = query.Where("tenant_id = ?", tenantID)
		}

		var configs []*RollbackConfig
		if err := query.Order("created_at DESC").Find(&configs).Error; err != nil {
			return nil, fmt.Errorf("list rollback configs: %w", err)
		}
		return configs, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*RollbackConfig
	for _, config := range e.configs {
		if agentID != "" && config.AgentID != agentID {
			continue
		}
		if tenantID != "" && config.TenantID != tenantID {
			continue
		}
		result = append(result, config)
	}
	return result, nil
}

// TakeSnapshot creates a configuration snapshot
func (e *Engine) TakeSnapshot(ctx context.Context, configID, snapshotData, version, description, createdBy string) (*ConfigSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	config, exists := e.configs[configID]
	if !exists {
		return nil, fmt.Errorf("rollback config not found")
	}

	snapshot := &ConfigSnapshot{
		ID:           uuid.New().String(),
		ConfigID:     configID,
		SnapshotData: snapshotData,
		Version:      version,
		Description:  description,
		CreatedAt:    time.Now(),
		CreatedBy:    createdBy,
		IsActive:     true,
	}

	if e.db != nil {
		if err := e.db.Create(snapshot).Error; err != nil {
			return nil, fmt.Errorf("create snapshot: %w", err)
		}
	}

	e.snapshots[configID] = append([]*ConfigSnapshot{snapshot}, e.snapshots[configID]...)

	// Trim old snapshots
	if len(e.snapshots[configID]) > config.MaxSnapshots {
		removed := e.snapshots[configID][config.MaxSnapshots:]
		e.snapshots[configID] = e.snapshots[configID][:config.MaxSnapshots]

		// Delete removed snapshots from database
		if e.db != nil {
			for _, r := range removed {
				e.db.Delete(r)
			}
		}
	}

	return snapshot, nil
}

// ListSnapshots lists snapshots for a configuration
func (e *Engine) ListSnapshots(ctx context.Context, configID string, limit int) ([]*ConfigSnapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	snapshots, exists := e.snapshots[configID]
	if !exists {
		if e.db != nil {
			var snaps []*ConfigSnapshot
			query := e.db.Where("config_id = ?", configID).Order("created_at DESC")
			if limit > 0 {
				query = query.Limit(limit)
			}
			if err := query.Find(&snaps).Error; err != nil {
				return nil, fmt.Errorf("list snapshots: %w", err)
			}
			return snaps, nil
		}
		return nil, fmt.Errorf("config not found")
	}

	if limit > 0 && len(snapshots) > limit {
		return snapshots[:limit], nil
	}
	return snapshots, nil
}

// ExecuteRollback executes a rollback to a specific snapshot
func (e *Engine) ExecuteRollback(ctx context.Context, configID, snapshotID, triggeredBy string) (*RollbackEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	config, exists := e.configs[configID]
	if !exists {
		return nil, fmt.Errorf("rollback config not found")
	}

	// Check cooldown
	events := e.events[configID]
	if len(events) > 0 {
		lastEvent := events[0]
		cooldownEnd := lastEvent.Timestamp.Add(time.Duration(config.CoolDownPeriod) * time.Minute)
		if time.Now().Before(cooldownEnd) && triggeredBy == "manual" {
			return nil, fmt.Errorf("cooldown period active, next rollback available at %v", cooldownEnd)
		}
	}

	// Find snapshot
	snapshot, err := e.findSnapshot(configID, snapshotID)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()

	// Parse snapshot data
	var configData interface{}
	if err := json.Unmarshal([]byte(snapshot.SnapshotData), &configData); err != nil {
		return nil, fmt.Errorf("parse snapshot data: %w", err)
	}

	// In a real implementation, this would restore the configuration
	// For now, we just record the event

	event := &RollbackEvent{
		ID:          uuid.New().String(),
		ConfigID:    configID,
		SnapshotID:  snapshotID,
		EventType:   "rollback",
		TriggeredBy: triggeredBy,
		FromVersion: getCurrentVersion(configID),
		ToVersion:   snapshot.Version,
		Success:     true,
		DurationMs:  time.Since(startTime).Milliseconds(),
		Timestamp:   time.Now(),
	}

	if e.db != nil {
		if err := e.db.Create(event).Error; err != nil {
			return nil, fmt.Errorf("create rollback event: %w", err)
		}

		// Mark snapshot as active
		e.db.Model(&ConfigSnapshot{}).Where("config_id = ?", configID).Update("is_active", false)
		e.db.Model(&ConfigSnapshot{}).Where("id = ?", snapshotID).Update("is_active", true)
	}

	e.events[configID] = append([]*RollbackEvent{event}, e.events[configID]...)

	return event, nil
}

// ListRollbackEvents lists rollback events
func (e *Engine) ListRollbackEvents(ctx context.Context, configID string, limit int) ([]*RollbackEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	events, exists := e.events[configID]
	if !exists {
		if e.db != nil {
			var evts []*RollbackEvent
			query := e.db.Where("config_id = ?", configID).Order("timestamp DESC")
			if limit > 0 {
				query = query.Limit(limit)
			}
			if err := query.Find(&evts).Error; err != nil {
				return nil, fmt.Errorf("list rollback events: %w", err)
			}
			return evts, nil
		}
		return nil, fmt.Errorf("config not found")
	}

	if limit > 0 && len(events) > limit {
		return events[:limit], nil
	}
	return events, nil
}

// GetStatus returns the rollback status for a configuration
func (e *Engine) GetStatus(ctx context.Context, configID string) (*RollbackStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	config, exists := e.configs[configID]
	if !exists {
		return nil, fmt.Errorf("rollback config not found")
	}

	status := &RollbackStatus{
		ConfigID:      configID,
		CurrentVersion: getCurrentVersion(configID),
		CanRollback:    true,
	}

	snapshots := e.snapshots[configID]
	if len(snapshots) > 0 {
		status.LastSnapshot = snapshots[0].CreatedAt
	}

	events := e.events[configID]
	if len(events) > 0 {
		status.LastRollback = events[0].Timestamp
		status.RollbackCount = int64(len(events))

		// Check cooldown
		cooldownEnd := events[0].Timestamp.Add(time.Duration(config.CoolDownPeriod) * time.Minute)
		if time.Now().Before(cooldownEnd) {
			status.CanRollback = false
			status.NextRollback = cooldownEnd
		}
	}

	return status, nil
}

// AutoRollback triggers automatic rollback when conditions are met
func (e *Engine) AutoRollback(ctx context.Context, configID, trigger string) (*RollbackEvent, error) {
	e.mu.RLock()
	config, exists := e.configs[configID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("rollback config not found")
	}

	if !config.AutoRollback {
		return nil, fmt.Errorf("auto rollback not enabled")
	}

	// Find last good snapshot
	snapshots := e.snapshots[configID]
	if len(snapshots) < 2 {
		return nil, fmt.Errorf("no previous snapshot available for rollback")
	}

	// Rollback to previous snapshot (skip current)
	previousSnapshot := snapshots[1]

	return e.ExecuteRollback(ctx, configID, previousSnapshot.ID, trigger)
}

// CheckSLOTrigger checks if SLO breach should trigger rollback
func (e *Engine) CheckSLOTrigger(ctx context.Context, configID string, currentSLO float64) (bool, error) {
	e.mu.RLock()
	config, exists := e.configs[configID]
	e.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("rollback config not found")
	}

	if !config.RollbackOnSLO {
		return false, nil
	}

	if currentSLO < config.SLOThreshold {
		// Trigger automatic rollback
		_, err := e.AutoRollback(ctx, configID, "slo_breach")
		return err == nil, err
	}

	return false, nil
}

// DeleteConfig deletes a rollback configuration and its snapshots
func (e *Engine) DeleteConfig(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		e.db.Where("config_id = ?", id).Delete(&ConfigSnapshot{})
		e.db.Where("config_id = ?", id).Delete(&RollbackEvent{})
		if err := e.db.Delete(&RollbackConfig{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete rollback config: %w", err)
		}
	}

	delete(e.configs, id)
	delete(e.snapshots, id)
	delete(e.events, id)
	return nil
}

// Helper functions

func (e *Engine) findSnapshot(configID, snapshotID string) (*ConfigSnapshot, error) {
	snapshots := e.snapshots[configID]
	for _, s := range snapshots {
		if s.ID == snapshotID {
			return s, nil
		}
	}

	if e.db != nil {
		var s ConfigSnapshot
		if err := e.db.First(&s, "id = ?", snapshotID).Error; err != nil {
			return nil, fmt.Errorf("snapshot not found")
		}
		return &s, nil
	}

	return nil, fmt.Errorf("snapshot not found")
}

func getCurrentVersion(configID string) string {
	// In a real implementation, this would get the current version from the target config
	return "current"
}