// Package catalog provides agent catalog management
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentStatus represents the status of a catalog agent
type AgentStatus string

const (
	AgentStatusActive    AgentStatus = "active"
	AgentStatusInactive  AgentStatus = "inactive"
	AgentStatusDeprecated AgentStatus = "deprecated"
)

// CatalogAgent represents an agent in the catalog
type CatalogAgent struct {
	ID            string       `gorm:"primaryKey"`
	Name          string       `gorm:"uniqueIndex"`
	Type          string       `gorm:"index"`
	Description   string
	Version       string
	Author        string
	Status        AgentStatus  `gorm:"index"`
	Configuration string       // JSON configuration
	Capabilities  string       // JSON capabilities
	Requirements  string       // JSON requirements
	Tags          string       // Comma-separated tags
	Rating        float64      // User rating
	UsageCount    int64
	LastUsed      *time.Time
	Metadata      string       // JSON additional metadata
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CatalogCategory represents a category in the catalog
type CatalogCategory struct {
	ID          string
	Name        string
	Description string
	ParentID    string
	AgentCount  int64
}

// Engine is the catalog engine
type Engine struct {
	db     *gorm.DB
	agents map[string]*CatalogAgent
	mu     sync.RWMutex
}

// NewEngine creates a new catalog engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:     db,
		agents: make(map[string]*CatalogAgent),
	}
	e.loadAgents()
	return e
}

// NewEngineMemory creates an in-memory catalog engine
func NewEngineMemory() *Engine {
	return &Engine{
		agents: make(map[string]*CatalogAgent),
	}
}

// loadAgents loads agents from database
func (e *Engine) loadAgents() {
	if e.db == nil {
		return
	}

	var agents []CatalogAgent
	if err := e.db.Find(&agents).Error; err != nil {
		return
	}

	for _, a := range agents {
		e.agents[a.ID] = &a
	}
}

// RegisterAgent registers an agent in the catalog
func (e *Engine) RegisterAgent(ctx context.Context, agent *CatalogAgent) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	if agent.Status == "" {
		agent.Status = AgentStatusActive
	}

	if e.db != nil {
		if err := e.db.Create(agent).Error; err != nil {
			return fmt.Errorf("register agent: %w", err)
		}
	}

	e.agents[agent.ID] = agent
	return nil
}

// GetCatalogAgent retrieves an agent from the catalog
func (e *Engine) GetCatalogAgent(ctx context.Context, id string) (*CatalogAgent, error) {
	e.mu.RLock()
	agent, exists := e.agents[id]
	e.mu.RUnlock()

	if exists {
		return agent, nil
	}

	if e.db != nil {
		var a CatalogAgent
		if err := e.db.First(&a, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get agent: %w", err)
		}
		return &a, nil
	}

	return nil, fmt.Errorf("agent not found")
}

// GetCatalogAgentByName retrieves an agent by name
func (e *Engine) GetCatalogAgentByName(ctx context.Context, name string) (*CatalogAgent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, agent := range e.agents {
		if agent.Name == name {
			return agent, nil
		}
	}

	if e.db != nil {
		var a CatalogAgent
		if err := e.db.First(&a, "name = ?", name).Error; err != nil {
			return nil, fmt.Errorf("get agent: %w", err)
		}
		return &a, nil
	}

	return nil, fmt.Errorf("agent not found")
}

// ListCatalogAgents lists agents in the catalog
func (e *Engine) ListCatalogAgents(ctx context.Context, agentType string, status AgentStatus) ([]*CatalogAgent, error) {
	if e.db != nil {
		query := e.db.Model(&CatalogAgent{})
		if agentType != "" {
			query = query.Where("type = ?", agentType)
		}
		if status != "" {
			query = query.Where("status = ?", status)
		}

		var agents []*CatalogAgent
		if err := query.Order("usage_count DESC, rating DESC").Find(&agents).Error; err != nil {
			return nil, fmt.Errorf("list agents: %w", err)
		}
		return agents, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*CatalogAgent
	for _, a := range e.agents {
		if agentType != "" && a.Type != agentType {
			continue
		}
		if status != "" && a.Status != status {
			continue
		}
		result = append(result, a)
	}
	return result, nil
}

// UpdateCatalogAgent updates an agent in the catalog
func (e *Engine) UpdateCatalogAgent(ctx context.Context, agent *CatalogAgent) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	agent.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Save(agent).Error; err != nil {
			return fmt.Errorf("update agent: %w", err)
		}
	}

	e.agents[agent.ID] = agent
	return nil
}

// DeleteCatalogAgent removes an agent from the catalog
func (e *Engine) DeleteCatalogAgent(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Delete(&CatalogAgent{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete agent: %w", err)
		}
	}

	delete(e.agents, id)
	return nil
}

// RefreshCatalog refreshes the catalog from source
func (e *Engine) RefreshCatalog(ctx context.Context) error {
	// In a real implementation, this would fetch from external sources
	return nil
}

// SearchAgents searches agents by query
func (e *Engine) SearchAgents(ctx context.Context, query string) ([]*CatalogAgent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*CatalogAgent
	for _, a := range e.agents {
		if a.Status != AgentStatusActive {
			continue
		}

		// Simple matching
		if contains(a.Name, query) || contains(a.Description, query) || contains(a.Tags, query) {
			result = append(result, a)
		}
	}
	return result, nil
}

// RecordUsage records usage of an agent
func (e *Engine) RecordUsage(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	agent, exists := e.agents[id]
	if !exists {
		return fmt.Errorf("agent not found")
	}

	agent.UsageCount++
	now := time.Now()
	agent.LastUsed = &now

	if e.db != nil {
		e.db.Model(agent).Updates(map[string]interface{}{
			"usage_count": agent.UsageCount,
			"last_used":   now,
		})
	}

	return nil
}

// RateAgent rates an agent
func (e *Engine) RateAgent(ctx context.Context, id string, rating float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	agent, exists := e.agents[id]
	if !exists {
		return fmt.Errorf("agent not found")
	}

	// Simple average rating
	if agent.Rating == 0 {
		agent.Rating = rating
	} else {
		agent.Rating = (agent.Rating + rating) / 2
	}

	if e.db != nil {
		e.db.Model(agent).Update("rating", agent.Rating)
	}

	return nil
}

// GetCategories gets catalog categories
func (e *Engine) GetCategories(ctx context.Context) ([]*CatalogCategory, error) {
	// Default categories
	categories := []*CatalogCategory{
		{ID: "browser", Name: "Browser Agents", Description: "Agents for web browsing and automation"},
		{ID: "chat", Name: "Chat Agents", Description: "Agents for conversational AI"},
		{ID: "code", Name: "Code Agents", Description: "Agents for code generation and analysis"},
		{ID: "data", Name: "Data Agents", Description: "Agents for data processing"},
		{ID: "research", Name: "Research Agents", Description: "Agents for research and information gathering"},
	}

	// Count agents in each category
	for _, cat := range categories {
		for _, agent := range e.agents {
			if agent.Type == cat.ID {
				cat.AgentCount++
			}
		}
	}

	return categories, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) && contains(s[1:], substr)))
}

// ExportCatalog exports the catalog to JSON
func (e *Engine) ExportCatalog(ctx context.Context) (string, error) {
	agents, err := e.ListCatalogAgents(ctx, "", "")
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return "", fmt.Errorf("export catalog: %w", err)
	}

	return string(data), nil
}

// ImportCatalog imports agents from JSON
func (e *Engine) ImportCatalog(ctx context.Context, catalogJSON string) (int, error) {
	var agents []*CatalogAgent
	if err := json.Unmarshal([]byte(catalogJSON), &agents); err != nil {
		return 0, fmt.Errorf("parse catalog: %w", err)
	}

	count := 0
	for _, a := range agents {
		if err := e.RegisterAgent(ctx, a); err != nil {
			continue
		}
		count++
	}

	return count, nil
}
