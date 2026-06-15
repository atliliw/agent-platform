// Package goldenpath provides golden path template management
package goldenpath

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TemplateType represents the type of template
type TemplateType string

const (
	TemplateTypeAgent    TemplateType = "agent"
	TemplateTypeWorkflow TemplateType = "workflow"
	TemplateTypePipeline TemplateType = "pipeline"
	TemplateTypeRule     TemplateType = "rule"
)

// Template represents a golden path template
type Template struct {
	ID          string       `gorm:"primaryKey"`
	Name        string       `gorm:"index"`
	Type        TemplateType `gorm:"index"`
	Description string
	Category    string       `gorm:"index"`
	Version     string
	Template    string       // JSON template definition
	Variables   string       // JSON variable definitions
	Examples    string       // JSON usage examples
	Tags        string       // Comma-separated tags
	Author      string
	IsPublic    bool         `gorm:"index"`
	UsageCount  int64
	TenantID    string       `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TemplateInstance represents an instantiated template
type TemplateInstance struct {
	ID         string    `gorm:"primaryKey"`
	TemplateID string    `gorm:"index"`
	Name       string
	Config     string    // JSON instantiated configuration
	Variables  string    // JSON variable values
	CreatedBy  string
	CreatedAt  time.Time
}

// Engine is the golden path template engine
type Engine struct {
	db        *gorm.DB
	templates map[string]*Template
	mu        sync.RWMutex
}

// NewEngine creates a new golden path engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:        db,
		templates: make(map[string]*Template),
	}
	e.loadTemplates()
	return e
}

// NewEngineMemory creates an in-memory golden path engine
func NewEngineMemory() *Engine {
	return &Engine{
		templates: make(map[string]*Template),
	}
}

// loadTemplates loads templates from database
func (e *Engine) loadTemplates() {
	if e.db == nil {
		return
	}

	var templates []Template
	if err := e.db.Find(&templates).Error; err != nil {
		return
	}

	for _, t := range templates {
		e.templates[t.ID] = &t
	}
}

// CreateTemplate creates a new template
func (e *Engine) CreateTemplate(ctx context.Context, template *Template) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if template.ID == "" {
		template.ID = uuid.New().String()
	}
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Create(template).Error; err != nil {
			return fmt.Errorf("create template: %w", err)
		}
	}

	e.templates[template.ID] = template
	return nil
}

// GetTemplate retrieves a template
func (e *Engine) GetTemplate(ctx context.Context, id string) (*Template, error) {
	e.mu.RLock()
	template, exists := e.templates[id]
	e.mu.RUnlock()

	if exists {
		return template, nil
	}

	if e.db != nil {
		var t Template
		if err := e.db.First(&t, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("get template: %w", err)
		}
		return &t, nil
	}

	return nil, fmt.Errorf("template not found")
}

// ListTemplates lists templates
func (e *Engine) ListTemplates(ctx context.Context, templateType TemplateType, category string) ([]*Template, error) {
	if e.db != nil {
		query := e.db.Model(&Template{})
		if templateType != "" {
			query = query.Where("type = ?", templateType)
		}
		if category != "" {
			query = query.Where("category = ?", category)
		}

		var templates []*Template
		if err := query.Order("usage_count DESC, created_at DESC").Find(&templates).Error; err != nil {
			return nil, fmt.Errorf("list templates: %w", err)
		}
		return templates, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Template
	for _, t := range e.templates {
		if templateType != "" && t.Type != templateType {
			continue
		}
		if category != "" && t.Category != category {
			continue
		}
		result = append(result, t)
	}
	return result, nil
}

// UpdateTemplate updates a template
func (e *Engine) UpdateTemplate(ctx context.Context, template *Template) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	template.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Save(template).Error; err != nil {
			return fmt.Errorf("update template: %w", err)
		}
	}

	e.templates[template.ID] = template
	return nil
}

// DeleteTemplate deletes a template
func (e *Engine) DeleteTemplate(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Delete(&Template{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete template: %w", err)
		}
	}

	delete(e.templates, id)
	return nil
}

// InstantiateTemplate instantiates a template with variables
func (e *Engine) InstantiateTemplate(ctx context.Context, templateID, name string, variables map[string]interface{}) (*TemplateInstance, error) {
	e.mu.RLock()
	template, exists := e.templates[templateID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("template not found")
	}

	// Parse template definition
	var templateDef map[string]interface{}
	if err := json.Unmarshal([]byte(template.Template), &templateDef); err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	// Substitute variables
	config := e.substituteVariables(templateDef, variables)

	configJSON, _ := json.Marshal(config)
	variablesJSON, _ := json.Marshal(variables)

	instance := &TemplateInstance{
		ID:         uuid.New().String(),
		TemplateID: templateID,
		Name:       name,
		Config:     string(configJSON),
		Variables:  string(variablesJSON),
		CreatedAt:  time.Now(),
	}

	if e.db != nil {
		if err := e.db.Create(instance).Error; err != nil {
			return nil, fmt.Errorf("create instance: %w", err)
		}

		// Increment usage count
		e.db.Model(template).Update("usage_count", template.UsageCount+1)
	}

	return instance, nil
}

// substituteVariables substitutes variables in template
func (e *Engine) substituteVariables(template map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range template {
		switch v := value.(type) {
		case string:
			// Check if it's a variable reference
			if len(v) > 2 && v[0] == '$' && v[1] == '{' && v[len(v)-1] == '}' {
				varName := v[2 : len(v)-1]
				if varValue, ok := variables[varName]; ok {
					result[key] = varValue
				} else {
					result[key] = v
				}
			} else {
				result[key] = v
			}
		case map[string]interface{}:
			result[key] = e.substituteVariables(v, variables)
		default:
			result[key] = v
		}
	}

	return result
}

// ImportTemplates imports templates from JSON
func (e *Engine) ImportTemplates(ctx context.Context, templatesJSON string) (int, error) {
	var templates []*Template
	if err := json.Unmarshal([]byte(templatesJSON), &templates); err != nil {
		return 0, fmt.Errorf("parse templates: %w", err)
	}

	count := 0
	for _, t := range templates {
		if t.ID == "" {
			t.ID = uuid.New().String()
		}
		t.CreatedAt = time.Now()
		t.UpdatedAt = time.Now()

		if err := e.CreateTemplate(ctx, t); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// ExportTemplates exports templates to JSON
func (e *Engine) ExportTemplates(ctx context.Context, templateType TemplateType) (string, error) {
	templates, err := e.ListTemplates(ctx, templateType, "")
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return "", fmt.Errorf("export templates: %w", err)
	}

	return string(data), nil
}

// SearchTemplates searches templates by name or tags
func (e *Engine) SearchTemplates(ctx context.Context, query string) ([]*Template, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Template
	for _, t := range e.templates {
		// Simple string matching
		if containsIgnoreCase(t.Name, query) ||
			containsIgnoreCase(t.Description, query) ||
			containsIgnoreCase(t.Tags, query) {
			result = append(result, t)
		}
	}
	return result, nil
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > 0 && len(substr) > 0 &&
				containsIgnoreCase(s[1:], substr) ||
				containsIgnoreCase(s, substr[1:])))
}
