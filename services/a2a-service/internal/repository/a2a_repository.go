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

	return r.registeredAgentToCard(&agent), nil
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
		cards = append(cards, r.registeredAgentToCard(a))
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

// registeredAgentToCard converts a RegisteredAgent DB row to a model.AgentCard,
// deserializing JSON fields back to slices.
func (r *A2ARepository) registeredAgentToCard(a *RegisteredAgent) *model.AgentCard {
	var capabilities []string
	if a.Capabilities != "" {
		json.Unmarshal([]byte(a.Capabilities), &capabilities)
	}

	var inputModes []string
	if a.InputModes != "" {
		json.Unmarshal([]byte(a.InputModes), &inputModes)
	}

	var outputModes []string
	if a.OutputModes != "" {
		json.Unmarshal([]byte(a.OutputModes), &outputModes)
	}

	return &model.AgentCard{
		ID:           a.ID,
		Name:         a.Name,
		Description:  a.Description,
		Capabilities: capabilities,
		InputModes:   inputModes,
		OutputModes:  outputModes,
		URL:          a.URL,
	}
}

// SeedDefaultAgents inserts default A2A agents if the database is empty.
func (r *A2ARepository) SeedDefaultAgents(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var count int64
	r.db.Model(&RegisteredAgent{}).Count(&count)
	if count > 0 {
		return 0, nil
	}

	defaults := []struct {
		ID           string
		Name         string
		Description  string
		Capabilities []string
		InputModes   []string
		OutputModes  []string
		URL          string
		TenantID     string
	}{
		{
			ID:           "gpt-researcher",
			Name:         "GPT Researcher",
			Description:  "自主研究 Agent，能对任何主题进行深度在线研究并生成详细报告",
			Capabilities: []string{"research", "web_search", "report_generation"},
			InputModes:   []string{"text"},
			OutputModes:  []string{"text", "markdown"},
			URL:          "http://gpt-researcher:8000",
			TenantID:     "default",
		},
		{
			ID:           "openhands",
			Name:         "OpenHands",
			Description:  "AI 软件开发 Agent，能编写代码、执行命令和浏览网页",
			Capabilities: []string{"code_generation", "bash_execution", "web_browsing", "file_editing"},
			InputModes:   []string{"text"},
			OutputModes:  []string{"text", "code"},
			URL:          "http://openhands:3000",
			TenantID:     "default",
		},
		{
			ID:           "auto-gpt",
			Name:         "AutoGPT",
			Description:  "自主 AI Agent，能分解目标为子任务并逐步执行，支持网页搜索和文件操作",
			Capabilities: []string{"planning", "web_search", "file_operations", "code_execution"},
			InputModes:   []string{"text"},
			OutputModes:  []string{"text", "json"},
			URL:          "http://auto-gpt:8000",
			TenantID:     "default",
		},
		{
			ID:           "crewai-agent",
			Name:         "CrewAI Agent",
			Description:  "多角色协作 Agent，支持角色定义、任务分配和团队协作完成复杂工作流",
			Capabilities: []string{"collaboration", "task_delegation", "role_based"},
			InputModes:   []string{"text", "json"},
			OutputModes:  []string{"text", "json"},
			URL:          "http://crewai:8000",
			TenantID:     "default",
		},
		{
			ID:           "browser-use-agent",
			Name:         "Browser Use Agent",
			Description:  "浏览器自动化 Agent，能自主操作网页完成表单填写、数据采集、页面导航等任务",
			Capabilities: []string{"web_browsing", "form_filling", "data_extraction", "screenshot"},
			InputModes:   []string{"text"},
			OutputModes:  []string{"text", "json", "image"},
			URL:          "http://browser-use:8000",
			TenantID:     "default",
		},
		{
			ID:           "local-agent-platform",
			Name:         "Local Agent Platform",
			Description:  "本地多 Agent 平台，具备 RAG 知识检索、多 Agent 编排和工具调用能力",
			Capabilities: []string{"chat", "search", "multi_agent", "tool_calling"},
			InputModes:   []string{"text", "json"},
			OutputModes:  []string{"text", "json"},
			URL:          "http://localhost:8080",
			TenantID:     "default",
		},
	}

	now := time.Now()
	inserted := 0
	for _, d := range defaults {
		capJSON, _ := json.Marshal(d.Capabilities)
		inJSON, _ := json.Marshal(d.InputModes)
		outJSON, _ := json.Marshal(d.OutputModes)

		agent := &RegisteredAgent{
			ID:           d.ID,
			Name:         d.Name,
			Description:  d.Description,
			Capabilities: string(capJSON),
			InputModes:   string(inJSON),
			OutputModes:  string(outJSON),
			URL:          d.URL,
			TenantID:     d.TenantID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := r.db.Create(agent).Error; err != nil {
			continue
		}
		inserted++
	}

	return inserted, nil
}

// Close closes the database connection
func (r *A2ARepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}