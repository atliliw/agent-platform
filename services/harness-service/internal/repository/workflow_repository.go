// Package repository provides data access for Workflow entities
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// WorkflowModel is the GORM model for workflow persistence
type WorkflowModel struct {
	ID          string `gorm:"primaryKey"`
	Name        string
	Description string
	Nodes       string // JSON-encoded nodes
	Edges       string // JSON-encoded edges
	EntryNodeID string
	TenantID    string `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WorkflowRepository provides CRUD operations for workflows
type WorkflowRepository struct {
	db *gorm.DB
	mu sync.RWMutex
}

// NewWorkflowRepository creates a new workflow repository with SQLite
func NewWorkflowRepository(dbPath string) (*WorkflowRepository, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open workflow database: %w", err)
	}

	if err := db.AutoMigrate(&WorkflowModel{}); err != nil {
		return nil, fmt.Errorf("failed to migrate workflow schema: %w", err)
	}

	return &WorkflowRepository{db: db}, nil
}

// NewWorkflowRepositoryWithDB creates a workflow repository using an existing DB
func NewWorkflowRepositoryWithDB(db *gorm.DB) *WorkflowRepository {
	db.AutoMigrate(&WorkflowModel{})
	return &WorkflowRepository{db: db}
}

// Save persists a workflow
func (r *WorkflowRepository) Save(ctx context.Context, wf *WorkflowModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if wf.ID == "" {
		wf.ID = uuid.New().String()
	}

	now := time.Now()
	if wf.CreatedAt.IsZero() {
		wf.CreatedAt = now
	}
	wf.UpdatedAt = now

	return r.db.WithContext(ctx).Save(wf).Error
}

// Get retrieves a workflow by ID
func (r *WorkflowRepository) Get(ctx context.Context, id string) (*WorkflowModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var wf WorkflowModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&wf).Error; err != nil {
		return nil, err
	}
	return &wf, nil
}

// List returns all workflows, optionally filtered by tenant
func (r *WorkflowRepository) List(ctx context.Context, tenantID string) ([]*WorkflowModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var workflows []*WorkflowModel
	query := r.db.WithContext(ctx)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Find(&workflows).Error; err != nil {
		return nil, err
	}
	return workflows, nil
}

// Delete removes a workflow by ID
func (r *WorkflowRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&WorkflowModel{}).Error
}

// SerializeNodes converts node data to JSON string for storage
func SerializeNodes(nodes interface{}) (string, error) {
	data, err := json.Marshal(nodes)
	if err != nil {
		return "", fmt.Errorf("failed to serialize nodes: %w", err)
	}
	return string(data), nil
}

// DeserializeNodes converts JSON string back to node data
func DeserializeNodes(data string, target interface{}) error {
	if data == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(data), target); err != nil {
		return fmt.Errorf("failed to deserialize nodes: %w", err)
	}
	return nil
}

// Close closes the database connection
func (r *WorkflowRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
