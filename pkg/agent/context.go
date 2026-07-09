package agent

import (
	"encoding/json"
	"time"
)

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`    // system, user, assistant, tool
	Content string `json:"content"` // Message content
	Name    string `json:"name,omitempty"`    // For tool messages
	ToolID  string `json:"tool_id,omitempty"` // Tool call ID if applicable
}

// ToolCall represents a tool call request
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Result   string          `json:"result,omitempty"`
	Status   string          `json:"status"` // pending, running, completed, error
}

// AgentExecutionRecord records a single agent execution step
type AgentExecutionRecord struct {
	AgentID     string    `json:"agent_id"`
	AgentName   string    `json:"agent_name"`
	Thought     string    `json:"thought"`           // Agent's reasoning
	Action      string    `json:"action"`            // Action taken (tool name or handoff)
	Arguments   string    `json:"arguments"`         // Action arguments
	Result      string    `json:"result"`            // Action result
	HandoffTo   string    `json:"handoff_to,omitempty"` // Target agent if handoff
	TokensUsed  int       `json:"tokens_used"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Duration    int64     `json:"duration_ms"` // Duration in milliseconds
}

	// TaskPlan is a decomposed task plan (todo list) the agent works through.
	type TaskPlan struct {
		Items     []TaskItem `json:"items"`
		UpdatedAt time.Time  `json:"updated_at"`
	}

	// TaskItem is a single todo item in a TaskPlan.
	type TaskItem struct {
		ID          string    `json:"id"`
		Description string    `json:"description"`
		Status      string    `json:"status"` // pending / in_progress / done / skipped
		AddedAt     time.Time `json:"added_at"`
	}

// ExecutionContext holds the shared state across agent executions
type ExecutionContext struct {
	// ID is the unique execution context ID
	ID string `json:"id"`

	// SessionID links to a user session
	SessionID string `json:"session_id"`

	// TenantID for multi-tenancy
	TenantID string `json:"tenant_id,omitempty"`

	// UserID is the user who initiated the execution
	UserID string `json:"user_id,omitempty"`

	// Variables are shared across agents
	Variables map[string]any `json:"variables"`

	// Messages is the conversation history
	Messages []Message `json:"messages"`

	// AgentHistory records all agent execution steps
	AgentHistory []AgentExecutionRecord `json:"agent_history"`

	// ToolResults caches tool execution results
	ToolResults map[string]ToolCall `json:"tool_results"`

	// CurrentAgent is the currently executing agent
	CurrentAgent string `json:"current_agent"`

	// EntryAgent is the initial agent
	EntryAgent string `json:"entry_agent"`

	// Status is the execution status
	Status AgentStatus `json:"status"`

	// TotalTokens used across all agents
	TotalTokens int `json:"total_tokens"`

	// TotalCost of execution
	TotalCost float64 `json:"total_cost"`

	// Error message if any
	Error string `json:"error,omitempty"`

	// StartedAt is when execution started
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when execution finished
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// StepCount is the current step number
	StepCount int `json:"step_count"`

	// SystemPromptOverride is the rendered prompt from Prompt Management.
	// When set, buildAgentMessages uses this instead of agent.Instructions.
	SystemPromptOverride string `json:"system_prompt_override,omitempty"`

	// Goal is the explicit task goal. When set, the agent is steered toward it
	// and a Verifier can check whether it was achieved.
	Goal string `json:"goal,omitempty"`

	// SuccessCriteria is a checkable completion condition. When set, the
	// done-branch runs the Verifier before declaring success.
	SuccessCriteria string `json:"success_criteria,omitempty"`

	// Plan is the decomposed task plan (todo list), produced by the planner
	// step and injected into the system prompt each step.
	Plan *TaskPlan `json:"plan,omitempty"`
}

// NewExecutionContext creates a new execution context
func NewExecutionContext(sessionID string) *ExecutionContext {
	return &ExecutionContext{
		ID:           generateContextID(),
		SessionID:    sessionID,
		Variables:    make(map[string]any),
		Messages:     []Message{},
		AgentHistory: []AgentExecutionRecord{},
		ToolResults:  make(map[string]ToolCall),
		Status:       AgentStatusIdle,
		StartedAt:    time.Now(),
	}
}

// generateContextID generates a unique context ID
func generateContextID() string {
	return "ctx_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

// randomString generates a random string of given length
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// SetVariable sets a context variable
func (c *ExecutionContext) SetVariable(key string, value any) {
	c.Variables[key] = value
}

// GetVariable gets a context variable
func (c *ExecutionContext) GetVariable(key string) (any, bool) {
	v, ok := c.Variables[key]
	return v, ok
}

// DeleteVariable deletes a context variable
func (c *ExecutionContext) DeleteVariable(key string) {
	delete(c.Variables, key)
}

// AddMessage adds a message to the conversation
func (c *ExecutionContext) AddMessage(role, content string) {
	c.Messages = append(c.Messages, Message{
		Role:    role,
		Content: content,
	})
}

// AddToolMessage adds a tool result message
func (c *ExecutionContext) AddToolMessage(toolName, toolID, content string) {
	c.Messages = append(c.Messages, Message{
		Role:    "tool",
		Content: content,
		Name:    toolName,
		ToolID:  toolID,
	})
}

// AddAgentRecord adds an agent execution record
func (c *ExecutionContext) AddAgentRecord(record AgentExecutionRecord) {
	c.AgentHistory = append(c.AgentHistory, record)
	c.StepCount++
}

// RecordToolCall records a tool call
func (c *ExecutionContext) RecordToolCall(tc ToolCall) {
	c.ToolResults[tc.ID] = tc
}

// GetCurrentAgent gets the current agent
func (c *ExecutionContext) GetCurrentAgent() string {
	return c.CurrentAgent
}

// SetCurrentAgent sets the current agent
func (c *ExecutionContext) SetCurrentAgent(agentID string) {
	c.CurrentAgent = agentID
}

// MarkRunning marks the context as running
func (c *ExecutionContext) MarkRunning() {
	c.Status = AgentStatusRunning
}

// MarkCompleted marks the context as completed
func (c *ExecutionContext) MarkCompleted() {
	c.Status = AgentStatusCompleted
	now := time.Now()
	c.CompletedAt = &now
}

// MarkError marks the context as errored
func (c *ExecutionContext) MarkError(err string) {
	c.Status = AgentStatusError
	c.Error = err
	now := time.Now()
	c.CompletedAt = &now
}

// AddTokens adds to the token count
func (c *ExecutionContext) AddTokens(tokens int) {
	c.TotalTokens += tokens
}

// AddCost adds to the cost
func (c *ExecutionContext) AddCost(cost float64) {
	c.TotalCost += cost
}

// GetLastAgentRecord gets the last agent execution record
func (c *ExecutionContext) GetLastAgentRecord() *AgentExecutionRecord {
	if len(c.AgentHistory) == 0 {
		return nil
	}
	return &c.AgentHistory[len(c.AgentHistory)-1]
}

// GetAgentRecordCount gets the count of records for a specific agent
func (c *ExecutionContext) GetAgentRecordCount(agentID string) int {
	count := 0
	for _, r := range c.AgentHistory {
		if r.AgentID == agentID {
			count++
		}
	}
	return count
}

// Clone creates a deep copy of the context
func (c *ExecutionContext) Clone() *ExecutionContext {
	clone := &ExecutionContext{
		ID:           c.ID,
		SessionID:    c.SessionID,
		TenantID:     c.TenantID,
		UserID:       c.UserID,
		CurrentAgent: c.CurrentAgent,
		EntryAgent:   c.EntryAgent,
		Status:       c.Status,
		TotalTokens:  c.TotalTokens,
		TotalCost:    c.TotalCost,
		Error:        c.Error,
		StartedAt:    c.StartedAt,
		StepCount:    c.StepCount,
		Goal:            c.Goal,
		SuccessCriteria: c.SuccessCriteria,
	}

	// Copy variables
	if c.Variables != nil {
		clone.Variables = make(map[string]any)
		for k, v := range c.Variables {
			clone.Variables[k] = v
		}
	}

	// Copy messages
	if c.Messages != nil {
		clone.Messages = make([]Message, len(c.Messages))
		copy(clone.Messages, c.Messages)
	}

	// Copy agent history
	if c.AgentHistory != nil {
		clone.AgentHistory = make([]AgentExecutionRecord, len(c.AgentHistory))
		copy(clone.AgentHistory, c.AgentHistory)
	}

	// Copy tool results
	if c.ToolResults != nil {
		clone.ToolResults = make(map[string]ToolCall)
		for k, v := range c.ToolResults {
			clone.ToolResults[k] = v
		}
	}

	// Copy completed at
	if c.CompletedAt != nil {
		t := *c.CompletedAt
		clone.CompletedAt = &t
	}

	// Copy plan
	if c.Plan != nil {
		clone.Plan = &TaskPlan{
			Items:     append([]TaskItem(nil), c.Plan.Items...),
			UpdatedAt: c.Plan.UpdatedAt,
		}
	}

	return clone
}

// ToJSON serializes the context to JSON
func (c *ExecutionContext) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON deserializes the context from JSON
func (c *ExecutionContext) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}
