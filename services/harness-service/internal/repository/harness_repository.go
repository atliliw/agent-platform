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
)

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
	ID           string    `gorm:"primaryKey"`
	Name         string
	ControlModel string
	VariantModel string
	TrafficSplit float64
	AgentID      string    `gorm:"index"`
	TenantID     string    `gorm:"index"`
	Status       string
	CreatedAt    time.Time
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

	if err := db.AutoMigrate(&Rule{}, &EvalSuite{}, &ABTest{}, &SLO{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &HarnessRepository{db: db}, nil
}

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

// Close closes the database connection
func (r *HarnessRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}