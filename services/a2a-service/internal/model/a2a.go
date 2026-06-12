// Package model provides data models for A2A service
package model

import (
	"time"
)

// AgentCard represents an agent's metadata
type AgentCard struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Capabilities []string          `json:"capabilities"`
	InputModes   []string          `json:"input_modes"`
	OutputModes  []string          `json:"output_modes"`
	URL          string            `json:"url"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// TaskStatus represents task status
type TaskStatus string

const (
	TaskStatusSubmitted TaskStatus = "submitted"
	TaskStatusWorking   TaskStatus = "working"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Message represents an A2A message
type Message struct {
	Role     string            `json:"role"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Task represents an A2A task
type Task struct {
	ID        string            `json:"id"`
	AgentID   string            `json:"agent_id"`
	Status    TaskStatus        `json:"status"`
	Messages  []Message         `json:"messages"`
	Result    string            `json:"result,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}