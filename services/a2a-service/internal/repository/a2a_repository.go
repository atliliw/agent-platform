// Package repository provides data access for A2A service
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

	"agent-platform/services/a2a-service/internal/model"
)

// RegisteredAgent represents a registered agent
type RegisteredAgent struct {
	ID           string    `gorm:"primaryKey"`
	Name         string
	Description  string
	Capabilities string // JSON array
	InputModes   string // JSON array
	OutputModes  string // JSON array
	URL          string
	TenantID     string    `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StoredTask represents a stored task
type StoredTask struct {
	ID        string    `gorm:"primaryKey"`
	AgentID   string    `gorm:"index"`
	Status    string
	Messages  string // JSON array
	Result    string
	Metadata  string // JSON object
	TenantID  string    `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// A2ARepository manages A2A data
type A2ARepository struct {
	db *gorm.DB
	mu sync.RWMutex
}

// NewA2ARepository creates a new A2A repository
func NewA2ARepository(dbPath string) (*A2ARepository, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.AutoMigrate(&RegisteredAgent{}, &StoredTask{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &A2ARepository{db: db}, nil
}

// RegisterAgent registers an agent
func (r *A2ARepository) RegisterAgent(ctx context.Context, card *model.AgentCard, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	capabilities, _ := json.Marshal(card.Capabilities)
	inputModes, _ := json.Marshal(card.InputModes)
	outputModes, _ := json.Marshal(card.OutputModes)

	agent := &RegisteredAgent{
		ID:           card.ID,
		Name:         card.Name,
		Description:  card.Description,
		Capabilities: string(capabilities),
		InputModes:   string(inputModes),
		OutputModes:  string(outputModes),
		URL:          card.URL,
		TenantID:     tenantID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return r.db.WithContext(ctx).Save(agent).Error
}

// UnregisterAgent unregisters an agent
func (r *A2ARepository) UnregisterAgent(ctx context.Context, agentID, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", agentID, tenantID).
		Delete(&RegisteredAgent{}).Error
}

// GetAgent gets an agent by ID
func (r *A2ARepository) GetAgent(ctx context.Context, agentID, tenantID string) (*model.AgentCard, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agent RegisteredAgent
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", agentID, tenantID).
		First(&agent).Error; err != nil {
		return nil, err
	}

	return &model.AgentCard{
		ID:          agent.ID,
		Name:        agent.Name,
		Description: agent.Description,
		URL:         agent.URL,
	}, nil
}

// ListAgents lists agents
func (r *A2ARepository) ListAgents(ctx context.Context, tenantID string) ([]*model.AgentCard, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agents []*RegisteredAgent
	query := r.db.WithContext(ctx)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Find(&agents).Error; err != nil {
		return nil, err
	}

	var cards []*model.AgentCard
	for _, a := range agents {
		cards = append(cards, &model.AgentCard{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			URL:         a.URL,
		})
	}

	return cards, nil
}

// CreateTask creates a task
func (r *A2ARepository) CreateTask(ctx context.Context, task *model.Task, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	messages, _ := json.Marshal(task.Messages)
	metadata, _ := json.Marshal(task.Metadata)

	stored := &StoredTask{
		ID:        task.ID,
		AgentID:   task.AgentID,
		Status:    string(task.Status),
		Messages:  string(messages),
		Result:    task.Result,
		Metadata:  string(metadata),
		TenantID:  tenantID,
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
	}

	return r.db.WithContext(ctx).Create(stored).Error
}

// GetTask gets a task by ID
func (r *A2ARepository) GetTask(ctx context.Context, taskID, tenantID string) (*model.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var stored StoredTask
	query := r.db.WithContext(ctx).Where("id = ?", taskID)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.First(&stored).Error; err != nil {
		return nil, err
	}

	var messages []model.Message
	json.Unmarshal([]byte(stored.Messages), &messages)

	var metadata map[string]string
	json.Unmarshal([]byte(stored.Metadata), &metadata)

	return &model.Task{
		ID:        stored.ID,
		AgentID:   stored.AgentID,
		Status:    model.TaskStatus(stored.Status),
		Messages:  messages,
		Result:    stored.Result,
		Metadata:  metadata,
		CreatedAt: stored.CreatedAt,
		UpdatedAt: stored.UpdatedAt,
	}, nil
}

// UpdateTask updates a task
func (r *A2ARepository) UpdateTask(ctx context.Context, task *model.Task, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	messages, _ := json.Marshal(task.Messages)
	metadata, _ := json.Marshal(task.Metadata)

	updates := map[string]interface{}{
		"status":     string(task.Status),
		"messages":   string(messages),
		"result":     task.Result,
		"metadata":   string(metadata),
		"updated_at": time.Now(),
	}

	return r.db.WithContext(ctx).
		Model(&StoredTask{}).
		Where("id = ? AND tenant_id = ?", task.ID, tenantID).
		Updates(updates).Error
}

// ListTasks lists tasks
func (r *A2ARepository) ListTasks(ctx context.Context, agentID, status, tenantID string, page, pageSize int) ([]*model.Task, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := r.db.WithContext(ctx).Model(&StoredTask{})
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var stored []*StoredTask
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&stored).Error; err != nil {
		return nil, 0, err
	}

	var tasks []*model.Task
	for _, s := range stored {
		var messages []model.Message
		json.Unmarshal([]byte(s.Messages), &messages)

		tasks = append(tasks, &model.Task{
			ID:        s.ID,
			AgentID:   s.AgentID,
			Status:    model.TaskStatus(s.Status),
			Messages:  messages,
			Result:    s.Result,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		})
	}

	return tasks, total, nil
}

// Close closes the database connection
func (r *A2ARepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}