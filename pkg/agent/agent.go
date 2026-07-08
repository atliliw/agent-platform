// Package agent provides multi-agent orchestration capabilities
package agent

import (
	"time"
)

// Agent represents an AI agent with specific capabilities
type Agent struct {
	// ID is the unique identifier for the agent
	ID string `json:"id" yaml:"id"`

	// Name is the human-readable name
	Name string `json:"name" yaml:"name"`

	// Description explains what this agent does
	Description string `json:"description" yaml:"description"`

	// Instructions is the system prompt for the agent (fallback when PromptTemplateKey is empty or harness is down)
	Instructions string `json:"instructions" yaml:"instructions"`

	// PromptTemplateKey references a template in Prompt Management.
	// When set, the engine uses the rendered template as the system prompt.
	// Instructions serves as fallback if rendering fails.
	PromptTemplateKey string `json:"prompt_template_key,omitempty" yaml:"prompt_template_key,omitempty"`

	// Tools are the tool names this agent can use
	Tools []string `json:"tools" yaml:"tools"`

	// Handoffs are the agent IDs this agent can transfer to
	Handoffs []string `json:"handoffs" yaml:"handoffs"`

	// Model is the LLM model to use (optional, defaults to system default)
	Model string `json:"model,omitempty" yaml:"model,omitempty"`

	// MaxTokens is the maximum tokens for responses
	MaxTokens int `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`

	// Temperature is the sampling temperature
	Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`

	// ToolConfig holds tool-specific configurations
	ToolConfig map[string]ToolSpecificConfig `json:"tool_config,omitempty" yaml:"tool_config,omitempty"`

	// Metadata contains additional custom properties
	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// CreatedAt is the creation timestamp
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`

	// UpdatedAt is the last update timestamp
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// ToolSpecificConfig holds tool-specific configuration
type ToolSpecificConfig struct {
	// APIKey for the tool (e.g., LLM API key for browser tool)
	APIKey string `json:"api_key,omitempty" yaml:"api_key,omitempty"`

	// BaseURL for the tool API
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`

	// Model for tool-specific LLM
	Model string `json:"model,omitempty" yaml:"model,omitempty"`

	// Additional config fields
	Extra map[string]any `json:"extra,omitempty" yaml:"extra,omitempty"`
}

// NewAgent creates a new agent with defaults
func NewAgent(id, name string) *Agent {
	return &Agent{
		ID:          id,
		Name:        name,
		Tools:       []string{},
		Handoffs:    []string{},
		Metadata:    make(map[string]any),
		MaxTokens:   4096,
		Temperature: 0.7,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// WithDescription sets the description
func (a *Agent) WithDescription(desc string) *Agent {
	a.Description = desc
	return a
}

// WithInstructions sets the instructions
func (a *Agent) WithInstructions(instructions string) *Agent {
	a.Instructions = instructions
	return a
}

// WithPromptTemplateKey sets the prompt template key for Prompt Management integration
func (a *Agent) WithPromptTemplateKey(key string) *Agent {
	a.PromptTemplateKey = key
	return a
}

// WithTools sets the available tools
func (a *Agent) WithTools(tools ...string) *Agent {
	a.Tools = tools
	return a
}

// AddTool adds a tool to the agent
func (a *Agent) AddTool(tool string) {
	a.Tools = append(a.Tools, tool)
}

// WithHandoffs sets the agents this agent can handoff to
func (a *Agent) WithHandoffs(agentIDs ...string) *Agent {
	a.Handoffs = agentIDs
	return a
}

// AddHandoff adds a handoff target
func (a *Agent) AddHandoff(agentID string) {
	a.Handoffs = append(a.Handoffs, agentID)
}

// WithModel sets the LLM model
func (a *Agent) WithModel(model string) *Agent {
	a.Model = model
	return a
}

// WithMetadata sets metadata
func (a *Agent) WithMetadata(key string, value any) *Agent {
	if a.Metadata == nil {
		a.Metadata = make(map[string]any)
	}
	a.Metadata[key] = value
	return a
}

// CanHandoffTo checks if this agent can handoff to the given agent
func (a *Agent) CanHandoffTo(agentID string) bool {
	for _, id := range a.Handoffs {
		if id == agentID {
			return true
		}
	}
	return false
}

// HasTool checks if this agent has the given tool
func (a *Agent) HasTool(toolName string) bool {
	for _, t := range a.Tools {
		if t == toolName {
			return true
		}
	}
	return false
}

// Validate validates the agent configuration
func (a *Agent) Validate() error {
	if a.ID == "" {
		return ErrAgentIDRequired
	}
	if a.Name == "" {
		return ErrAgentNameRequired
	}
	if a.Instructions == "" && a.PromptTemplateKey == "" {
		return ErrAgentInstructionsRequired
	}
	return nil
}

// Clone creates a deep copy of the agent
func (a *Agent) Clone() *Agent {
	clone := &Agent{
		ID:                a.ID,
		Name:              a.Name,
		Description:       a.Description,
		Instructions:      a.Instructions,
		PromptTemplateKey: a.PromptTemplateKey,
		Model:             a.Model,
		MaxTokens:         a.MaxTokens,
		Temperature:       a.Temperature,
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         time.Now(),
	}

	// Copy tools
	if a.Tools != nil {
		clone.Tools = make([]string, len(a.Tools))
		copy(clone.Tools, a.Tools)
	}

	// Copy handoffs
	if a.Handoffs != nil {
		clone.Handoffs = make([]string, len(a.Handoffs))
		copy(clone.Handoffs, a.Handoffs)
	}

	// Copy metadata
	if a.Metadata != nil {
		clone.Metadata = make(map[string]any)
		for k, v := range a.Metadata {
			clone.Metadata[k] = v
		}
	}

	// Copy tool config
	if a.ToolConfig != nil {
		clone.ToolConfig = make(map[string]ToolSpecificConfig)
		for k, v := range a.ToolConfig {
			clone.ToolConfig[k] = v
		}
	}

	return clone
}

// AgentStatus represents the status of an agent execution
type AgentStatus string

const (
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusError     AgentStatus = "error"
)
