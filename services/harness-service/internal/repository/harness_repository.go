// Package repository provides data access for Harness service
package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"agent-platform/services/harness-service/internal/slo"
)

// ==================== 基础模型 ====================

// Rule represents a harness rule
type Rule struct {
	ID        string    `gorm:"primaryKey"`
	AgentID   string    `gorm:"index"`
	Name      string
	Type      string
	Config    string
	Enabled   bool
	TenantID  string    `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EvalSuite represents an evaluation suite
type EvalSuite struct {
	ID          string    `gorm:"primaryKey"`
	Name        string
	Description string
	TenantID    string    `gorm:"index"`
	CreatedAt   time.Time
}

// ABTest represents an A/B test
type ABTest struct {
	ID            string    `gorm:"primaryKey"`
	Name          string
	ControlModel  string
	VariantModel  string
	TrafficSplit  float64
	AgentID       string    `gorm:"index"`
	TenantID      string    `gorm:"index"`
	Status        string
	Type          string    // "model" 或 "prompt"
	ControlConfig string    // 对照组配置
	VariantConfig string    // 实验组配置
	CreatedAt     time.Time
}

// SLO represents a service level objective
type SLO struct {
	ID        string    `gorm:"primaryKey"`
	AgentID   string    `gorm:"index"`
	Name      string
	Target    float64
	Type      string
	TenantID  string    `gorm:"index"`
	CreatedAt time.Time
}

// SLODefinition is used for SLO manager
type SLODefinition struct {
	ID             string
	AgentID        string
	Name           string
	Type           string
	Target         float64
	Window         string
	AlertThreshold float64
}

// ==================== Feature Flag 模型 ====================

// FeatureFlag represents a feature flag
type FeatureFlag struct {
	ID          string    `gorm:"primaryKey"`
	Key         string    `gorm:"uniqueIndex"`
	Name        string
	Description string
	Type        string    // "boolean", "string", "number", "json"
	Value       string    // Default value (JSON encoded)
	Status      string    `gorm:"index"` // "active", "inactive", "archived"
	Rules       string    // JSON encoded targeting rules
	Rollout     float64   // Percentage rollout (0-100)
	LastUsed    time.Time // Last time flag was evaluated
	TenantID    string    `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ==================== Rollback 模型 ====================

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

// ==================== RCA 模型 ====================

// ChangeEvent represents a recorded change
type ChangeEvent struct {
	ID          string    `gorm:"primaryKey"`
	AgentID     string    `gorm:"index"`
	ChangeType  string    `gorm:"index"` // "config", "deployment", "model", "rule", "feature_flag"
	ResourceID  string    `gorm:"index"`
	ResourceType string
	Description string
	OldValue    string    // JSON encoded
	NewValue    string    // JSON encoded
	Timestamp   time.Time `gorm:"index"`
	User        string
	Source      string    // "manual", "auto", "api"
	Metadata    string    // JSON encoded additional metadata
	TenantID    string    `gorm:"index"`
}

// IncidentEvent represents an incident or issue
type IncidentEvent struct {
	ID          string    `gorm:"primaryKey"`
	AgentID     string    `gorm:"index"`
	Title       string
	Description string
	Severity    string    `gorm:"index"` // "critical", "high", "medium", "low"
	Impact      string
	DetectedAt  time.Time `gorm:"index"`
	ResolvedAt  *time.Time
	Status      string    // "active", "resolved", "investigating"
	Metadata    string
	TenantID    string    `gorm:"index"`
}

// ==================== Chaos 模型 ====================

// ChaosExperiment represents a chaos experiment
type ChaosExperiment struct {
	ID             string    `gorm:"primaryKey"`
	Name           string
	Description    string
	AgentID        string    `gorm:"index"`
	FaultType      string    `gorm:"index"`
	FaultConfig    string    // JSON configuration for fault
	Duration       int       // Duration in minutes
	BlastRadius    float64   // 0-1, percentage of traffic to affect
	AutoStopOnSLO  bool      // Stop if SLO breach detected
	SLOThreshold   float64   // SLO threshold for auto-stop
	Status         string    `gorm:"index"` // "created", "running", "paused", "completed", "failed", "stopped"
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	EndedAt        *time.Time
	TenantID       string    `gorm:"index"`
}

// ChaosExperimentRun represents a single run of a chaos experiment
type ChaosExperimentRun struct {
	ID              string    `gorm:"primaryKey"`
	ExperimentID    string    `gorm:"index"`
	Status          string
	StartedAt       time.Time
	EndedAt         *time.Time
	FaultsInjected  int64
	RequestsAffected int64
	AutoStopped     bool
	SLOBreachAt     *time.Time
	Result          string    // JSON result summary
}

// ==================== Cost 模型 ====================

// ModelPricing represents pricing for a model
type ModelPricing struct {
	ID               string    `gorm:"primaryKey"`
	ModelID          string    `gorm:"uniqueIndex"`
	ModelName        string
	Provider         string
	InputPricePer1M  float64   // Price per 1M input tokens
	OutputPricePer1M float64   // Price per 1M output tokens
	Currency         string
	EffectiveFrom    time.Time
	EffectiveTo      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
	Currency     string
	Timestamp    time.Time `gorm:"index"`
	Metadata     string    // JSON
}

// ==================== Evolve 模型 ====================

// Proposal represents a self-evolution proposal
type Proposal struct {
	ID              string    `gorm:"primaryKey"`
	AgentID         string    `gorm:"index"`
	Type            string    `gorm:"index"` // "model_switch", "config_optimize", "cost_reduce", "performance", "ab_test"
	Title           string
	Description     string
	CurrentState    string    // JSON
	ProposedState   string    // JSON
	ExpectedBenefit float64   // Estimated benefit score
	RiskLevel       string    // "low", "medium", "high"
	Status          string    `gorm:"index"` // "pending", "approved", "rejected", "running", "completed", "failed"
	ApprovedBy      string
	ApprovedAt      *time.Time
	ExecutedAt      *time.Time
	Result          string    // JSON execution result
	Metadata        string    // JSON
	TenantID        string    `gorm:"index"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ==================== Golden Path 模型 ====================

// GoldenPathTemplate represents a golden path template
type GoldenPathTemplate struct {
	ID          string    `gorm:"primaryKey"`
	Name        string    `gorm:"index"`
	Type        string    `gorm:"index"` // "agent", "workflow", "pipeline", "rule"
	Description string
	Category    string    `gorm:"index"`
	Version     string
	Template    string    // JSON template definition
	Variables   string    // JSON variable definitions
	Examples    string    // JSON usage examples
	Tags        string    // Comma-separated tags
	Author      string
	IsPublic    bool      `gorm:"index"`
	UsageCount  int64
	TenantID    string    `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ==================== Catalog 模型 ====================

// CatalogAgent represents an agent in the catalog
type CatalogAgent struct {
	ID            string    `gorm:"primaryKey"`
	Name          string    `gorm:"uniqueIndex"`
	Type          string    `gorm:"index"`
	Description   string
	Version       string
	Author        string
	Status        string    `gorm:"index"` // "active", "inactive", "deprecated"
	Configuration string    // JSON configuration
	Capabilities  string    // JSON capabilities
	Requirements  string    // JSON requirements
	Tags          string    // Comma-separated tags
	Rating        float64   // User rating
	UsageCount    int64
	LastUsed      *time.Time
	Metadata      string    // JSON additional metadata
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ==================== Coordinate 模型 ====================

// OrchestrationRun represents an orchestration execution
type OrchestrationRun struct {
	ID           string    `gorm:"primaryKey"`
	AgentID      string    `gorm:"index"`
	Type         string    `gorm:"index"` // "sequential", "parallel", "conditional", "loop"
	Status       string    // "running", "completed", "failed"
	Steps        string    // JSON step definitions
	Results      string    // JSON step results
	Score        float64   // Quality score
	LatencyMs    int64     // Total latency
	TokenCount   int64     // Total tokens
	Cost         float64   // Total cost
	SuccessCount int       // Successful steps
	FailCount    int       // Failed steps
	StartedAt    time.Time
	EndedAt      *time.Time
	Metadata     string
}

// ==================== Planner 模型 ====================

// Plan represents an execution plan
type Plan struct {
	ID          string    `gorm:"primaryKey"`
	AgentID     string    `gorm:"index"`
	Goal        string
	Steps       string    // JSON step definitions
	Status      string    `gorm:"index"` // "draft", "executing", "completed", "failed"
	Score       float64   // Plan quality score
	ExecutionMs int64     // Execution time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ExecutedAt  *time.Time
}

// ==================== Prompt 模型 ====================

// Prompt represents a prompt template
type Prompt struct {
	ID          string    `gorm:"primaryKey"`
	Key         string    `gorm:"uniqueIndex"`
	Name        string
	Description string
	Category    string    `gorm:"index"` // "system", "user", "template", "rag", "agent"
	Tags        string    // JSON array
	TenantID    string    `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
}

// PromptVersion represents a specific version of a prompt
type PromptVersion struct {
	ID        string    `gorm:"primaryKey"`
	PromptID  string    `gorm:"index"`
	Version   string    // Semantic version
	Content   string    // Prompt content with {{var}} placeholders
	Variables string    // JSON schema for variables
	Metadata  string    // JSON metadata
	Status    string    `gorm:"index"` // "draft", "active", "archived"
	IsActive  bool      `gorm:"index"`
	CreatedAt time.Time
	CreatedBy string
}

// PromptPerformance represents performance metrics for a prompt version
type PromptPerformance struct {
	ID              string    `gorm:"primaryKey"`
	VersionID       string    `gorm:"index"`
	TotalCalls      int64
	SuccessCalls    int64
	SuccessRate     float64
	AvgLatency      float64
	AvgInputTokens  int64
	AvgOutputTokens int64
	AvgTotalTokens  int64
	AvgCost         float64
	UserRating      float64
	FeedbackCount   int64
	PeriodStart     time.Time `gorm:"index"`
	PeriodEnd       time.Time
}

// PromptUsageRecord represents a single usage event for performance tracking
type PromptUsageRecord struct {
	ID           string    `gorm:"primaryKey"`
	VersionID    string    `gorm:"index"`
	SessionID    string    `gorm:"index"`
	Success      bool
	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	UserRating   float64
	Timestamp    time.Time `gorm:"index"`
	Metadata     string    // JSON
}

// ==================== Repository ====================

// HarnessRepository manages harness data
type HarnessRepository struct {
	db *gorm.DB
	mu sync.RWMutex
}

// NewHarnessRepository creates a new harness repository
func NewHarnessRepository(dbPath string) (*HarnessRepository, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Auto migrate all models
	if err := db.AutoMigrate(
		// Basic models
		&Rule{}, &EvalSuite{}, &ABTest{}, &SLO{},
		// Feature Flag
		&FeatureFlag{},
		// Rollback
		&RollbackConfig{}, &ConfigSnapshot{}, &RollbackEvent{},
		// RCA
		&ChangeEvent{}, &IncidentEvent{},
		// Chaos
		&ChaosExperiment{}, &ChaosExperimentRun{},
		// Cost
		&ModelPricing{}, &UsageRecord{},
		// Evolve
		&Proposal{},
		// Golden Path
		&GoldenPathTemplate{},
		// Catalog
		&CatalogAgent{},
		// Coordinate
		&OrchestrationRun{},
		// Planner
		&Plan{},
		// Prompt
		&Prompt{}, &PromptVersion{}, &PromptPerformance{}, &PromptUsageRecord{},
		// SLO
		&slo.SLODefinition{},
		&slo.SLOEvent{},
	); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &HarnessRepository{db: db}, nil
}

// ==================== Basic CRUD Operations ====================

// CreateRule creates a rule
func (r *HarnessRepository) CreateRule(ctx context.Context, rule *Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	return r.db.WithContext(ctx).Create(rule).Error
}

// ListRules lists rules
func (r *HarnessRepository) ListRules(ctx context.Context, agentID, tenantID string) ([]*Rule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var rules []*Rule
	query := r.db.WithContext(ctx)
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Find(&rules).Error; err != nil {
		return nil, err
	}

	return rules, nil
}

// UpdateRule updates a rule
func (r *HarnessRepository) UpdateRule(ctx context.Context, rule *Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rule.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(rule).Error
}

// DeleteRule deletes a rule
func (r *HarnessRepository) DeleteRule(ctx context.Context, id, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&Rule{}).Error
}

// CreateEvalSuite creates an eval suite
func (r *HarnessRepository) CreateEvalSuite(ctx context.Context, suite *EvalSuite) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if suite.ID == "" {
		suite.ID = uuid.New().String()
	}
	suite.CreatedAt = time.Now()
	return r.db.WithContext(ctx).Create(suite).Error
}

// CreateABTest creates an A/B test
func (r *HarnessRepository) CreateABTest(ctx context.Context, test *ABTest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if test.ID == "" {
		test.ID = uuid.New().String()
	}
	test.CreatedAt = time.Now()
	return r.db.WithContext(ctx).Create(test).Error
}

// ListABTests lists A/B tests
func (r *HarnessRepository) ListABTests(ctx context.Context, agentID, tenantID, status string) ([]*ABTest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := r.db.WithContext(ctx).Model(&ABTest{})
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var tests []*ABTest
	if err := query.Order("created_at DESC").Find(&tests).Error; err != nil {
		return nil, err
	}
	return tests, nil
}

// DeleteABTest deletes an A/B test
func (r *HarnessRepository) DeleteABTest(ctx context.Context, id, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete by ID only — ID is a UUID primary key, sufficient for uniqueness
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&ABTest{}).Error
}

// CreateSLO creates an SLO
func (r *HarnessRepository) CreateSLO(ctx context.Context, slo *SLO) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if slo.ID == "" {
		slo.ID = uuid.New().String()
	}
	slo.CreatedAt = time.Now()
	return r.db.WithContext(ctx).Create(slo).Error
}

// GetDB returns the database instance
func (r *HarnessRepository) GetDB() *gorm.DB {
	return r.db
}

// Close closes the database connection
func (r *HarnessRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
