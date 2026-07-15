// Package prompt provides prompt version management with template rendering
package prompt

import (
	"time"
)

// PromptCategory represents the category of a prompt
type PromptCategory string

const (
	CategorySystem   PromptCategory = "system"
	CategoryUser     PromptCategory = "user"
	CategoryTemplate PromptCategory = "template"
	CategoryRAG      PromptCategory = "rag"
	CategoryAgent    PromptCategory = "agent"
)

// VersionStatus represents the status of a prompt version
type VersionStatus string

const (
	VersionStatusDraft   VersionStatus = "draft"
	VersionStatusActive  VersionStatus = "active"
	VersionStatusArchived VersionStatus = "archived"
)

// Prompt represents a prompt template with metadata
type Prompt struct {
	ID          string         `gorm:"primaryKey"`
	Key         string         `gorm:"uniqueIndex"` // Unique identifier for lookup
	Name        string
	Description string
	Category    PromptCategory `gorm:"index"`
	Tags        string         // JSON array of tags
	TenantID    string         `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
}

// PromptVersion represents a specific version of a prompt
type PromptVersion struct {
	ID        string        `gorm:"primaryKey"`
	PromptID  string        `gorm:"index"`
	Version   string        // Semantic version (e.g., "1.0.0")
	Content   string        // Prompt content with {{var}} placeholders
	Variables string        // JSON schema for variables
	Metadata  string        // JSON metadata (author, notes, etc.)
	Status    VersionStatus `gorm:"index"`
	IsActive  bool          `gorm:"index"`
	CreatedAt time.Time
	CreatedBy string
}

// PromptPerformance tracks performance metrics for a prompt version
type PromptPerformance struct {
	ID              string    `gorm:"primaryKey"`
	VersionID       string    `gorm:"index"`
	TotalCalls      int64
	SuccessCalls    int64
	SuccessRate     float64
	AvgLatency      float64   // milliseconds
	AvgInputTokens  int64
	AvgOutputTokens int64
	AvgTotalTokens  int64
	AvgCost         float64
	UserRating      float64   // Average user rating (1-5)
	FeedbackCount   int64
	PeriodStart     time.Time `gorm:"index"`
	PeriodEnd       time.Time
}

// Variable represents a template variable definition
type Variable struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`        // "string", "number", "boolean", "array", "object"
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description,omitempty"`
	Enum        []string    `json:"enum,omitempty"` // Allowed values
	Min         *float64    `json:"min,omitempty"`  // For number type
	Max         *float64    `json:"max,omitempty"`  // For number type
}

// VariableSet is a collection of variables
type VariableSet struct {
	Variables []Variable `json:"variables"`
}

// RenderContext provides context for rendering a prompt
type RenderContext struct {
	Variables map[string]interface{}
	UserID    string
	SessionID string
	AgentID   string
	Model     string
	Metadata  map[string]string
}

// VersionDiff represents the difference between two versions
type VersionDiff struct {
	Version1    string
	Version2    string
	ContentDiff []DiffLine
	VarDiff     []VarDiff
	Summary     string
}

// DiffLine represents a line in a diff
type DiffLine struct {
	Type    string // "add", "remove", "unchanged"
	Content string
}

// VarDiff represents a variable difference
type VarDiff struct {
	Name     string
	Type     string // "added", "removed", "changed"
	OldValue interface{}
	NewValue interface{}
}

// PerformanceTrend represents performance metrics over time
type PerformanceTrend struct {
	VersionID   string
	DataPoints  []PerformanceDataPoint
	Trend       string // "improving", "declining", "stable"
	ChangeRate  float64
}

// PerformanceDataPoint represents a single performance measurement
type PerformanceDataPoint struct {
	Timestamp    time.Time
	SuccessRate  float64
	AvgLatency   float64
	AvgCost      float64
	UserRating   float64
	CallCount    int64
}

// UsageRecord represents a single usage event for performance tracking
type UsageRecord struct {
	ID           string    `gorm:"primaryKey"`
	VersionID    string    `gorm:"index"`
	SessionID    string    `gorm:"index"`
	Success      bool
	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	UserRating   float64   // 0 if no rating provided
	Timestamp    time.Time `gorm:"index"`
	Metadata     string    // JSON
}
