package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NodeResultDetail holds per-node execution result details
type NodeResultDetail struct {
	NodeID   string `json:"node_id"`
	NodeName string `json:"node_name,omitempty"`
	NodeType string `json:"node_type,omitempty"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Duration int64  `json:"duration_ms,omitempty"`
	Retries  int    `json:"retries,omitempty"`
	Status   string `json:"status,omitempty"`
}

// ExecutionRecord is the GORM model for workflow execution history
type ExecutionRecord struct {
	ID           string     `gorm:"primaryKey"`
	WorkflowID   string     `gorm:"index"`
	Status       string     `gorm:"index"`
	Input        string
	FinalOutput  string
	Error        string
	NodeResults  string     // JSON-encoded []NodeResultDetail
	TenantID     string     `gorm:"index"`
	StartedAt    time.Time
	CompletedAt  *time.Time
	Duration     int64      // milliseconds
}

// ExecutionRepository provides CRUD for workflow execution records
type ExecutionRepository struct {
	db *gorm.DB
	mu sync.RWMutex
}

// NewExecutionRepositoryWithDB creates an execution repository using an existing DB
func NewExecutionRepositoryWithDB(db *gorm.DB) *ExecutionRepository {
	db.AutoMigrate(&ExecutionRecord{})
	return &ExecutionRepository{db: db}
}

// Save persists an execution record
func (r *ExecutionRepository) Save(ctx context.Context, exec *ExecutionRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if exec.ID == "" {
		exec.ID = uuid.New().String()
	}
	return r.db.WithContext(ctx).Save(exec).Error
}

// Get retrieves an execution by ID
func (r *ExecutionRepository) Get(ctx context.Context, id string) (*ExecutionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var exec ExecutionRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&exec).Error; err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}
	return &exec, nil
}

// ListByWorkflow returns executions for a workflow, ordered by most recent first
func (r *ExecutionRepository) ListByWorkflow(ctx context.Context, workflowID string, limit int) ([]*ExecutionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	var executions []*ExecutionRecord
	if err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("started_at DESC").
		Limit(limit).
		Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	return executions, nil
}

// ListRecent returns recent executions across all workflows
func (r *ExecutionRepository) ListRecent(ctx context.Context, limit int) ([]*ExecutionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var executions []*ExecutionRecord
	if err := r.db.WithContext(ctx).
		Order("started_at DESC").
		Limit(limit).
		Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("list recent executions: %w", err)
	}
	return executions, nil
}

// UpdateStatus updates the status and completion fields of an execution
func (r *ExecutionRepository) UpdateStatus(ctx context.Context, id string, status string, finalOutput string, errMsg string, nodeResults string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	updates := map[string]interface{}{
		"status":       status,
		"final_output": finalOutput,
		"error":        errMsg,
		"node_results": nodeResults,
	}

	if status == string(StatusCompleted) || status == string(StatusFailed) || status == string(StatusCancelled) || status == string(StatusTimedOut) {
		now := time.Now()
		updates["completed_at"] = &now
	}

	return r.db.WithContext(ctx).Model(&ExecutionRecord{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes an execution record
func (r *ExecutionRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&ExecutionRecord{}).Error
}
