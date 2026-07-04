// Package session provides session replay functionality
package session

import (
	"time"
)

// StepType defines the type of session step
type StepType string

const (
	StepTypeThink     StepType = "think"
	StepTypeToolCall  StepType = "tool_call"
	StepTypeAction    StepType = "action"
	StepTypeObservation StepType = "observation"
	StepTypeDecision  StepType = "decision"
	StepTypeLLMCall   StepType = "llm_call"
)

// SessionStatus defines the status of a session
type SessionStatus string

const (
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
)

// StepStatus defines the status of a step
type StepStatus string

const (
	StepStatusSuccess StepStatus = "success"
	StepStatusFailed  StepStatus = "failed"
	StepStatusPending StepStatus = "pending"
)

// Session represents a complete agent execution session
type Session struct {
	ID          string            `json:"id" gorm:"primaryKey"`
	AgentID     string            `json:"agent_id" gorm:"index"`
	TraceID     string            `json:"trace_id" gorm:"index"`
	Status      SessionStatus     `json:"status" gorm:"index"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     *time.Time        `json:"end_time"`
	Duration    int64             `json:"duration"` // milliseconds
	TotalTokens int64             `json:"total_tokens"`
	TotalCost   float64           `json:"total_cost"`
	Model       string            `json:"model"`
	Metadata    map[string]string `json:"metadata" gorm:"-"`
	MetadataJSON string           `json:"-" gorm:"column:metadata"` // JSON encoded
	TenantID    string            `json:"tenant_id" gorm:"index"`
	CreatedAt   time.Time         `json:"created_at"`
}

// SessionStep represents a single step in a session
type SessionStep struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	SessionID    string     `json:"session_id" gorm:"index"`
	StepNumber   int32      `json:"step_number"`
	StepType     StepType   `json:"step_type" gorm:"index"`
	ParentStepID string     `json:"parent_step_id" gorm:"index"` // For hierarchical steps
	Input        string     `json:"input"`    // JSON encoded input
	Output       string     `json:"output"`   // JSON encoded output
	Metadata     string     `json:"metadata"` // JSON encoded additional info
	Duration     int64      `json:"duration"` // milliseconds
	Status       StepStatus `json:"status" gorm:"index"`
	Timestamp    time.Time  `json:"timestamp" gorm:"index"`
}

// GraphNode represents a node in the execution graph
type GraphNode struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Label    string            `json:"label"`
	Duration int64             `json:"duration"`
	Status   string            `json:"status"`
	Metadata map[string]string `json:"metadata"`
}

// GraphEdge represents an edge in the execution graph
type GraphEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Label  string `json:"label"`
}

// SessionGraph represents the execution graph of a session
type SessionGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// ReplayDiff represents a difference between original and replayed output
type ReplayDiff struct {
	StepID          string `json:"step_id"`
	StepNumber      int32  `json:"step_number"`
	OriginalOutput  string `json:"original_output"`
	ReplayOutput    string `json:"replay_output"`
	Matches         bool   `json:"matches"`
}

// ReplaySession represents a replay execution
type ReplaySession struct {
	ID        string        `json:"id" gorm:"primaryKey"`
	SessionID string        `json:"session_id" gorm:"index"`
	FromStep  int32         `json:"from_step"`
	ToStep    int32         `json:"to_step"`
	DryRun    bool          `json:"dry_run"`
	Success   bool          `json:"success"`
	Error     string        `json:"error"`
	Diffs     []ReplayDiff  `json:"diffs" gorm:"-"`
	DiffsJSON string        `json:"-" gorm:"column:diffs"` // JSON encoded
	CreatedAt time.Time     `json:"created_at"`
	EndedAt   *time.Time    `json:"ended_at"`
}

// SessionDetail represents a session with its steps and graph
type SessionDetail struct {
	Session Session      `json:"session"`
	Steps   []SessionStep `json:"steps"`
	Graph   SessionGraph `json:"graph"`
}

// ListSessionsFilter defines filters for listing sessions
type ListSessionsFilter struct {
	AgentID   string `json:"agent_id"`
	Status    string `json:"status"`
	StartTime int64  `json:"start_time"` // Unix timestamp
	EndTime   int64  `json:"end_time"`   // Unix timestamp
	Page      int32  `json:"page"`
 PageSize  int32  `json:"page_size"`
	TenantID  string `json:"tenant_id"`
}